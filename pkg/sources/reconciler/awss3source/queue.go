/*
Copyright 2022 TriggerMesh Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awss3source

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go/aws/arn"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"

	"github.com/triggermesh/scoby-hook-triggermesh/pkg/apis"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/apis/sources/v1alpha1"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/sources/aws/iam"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/sources/aws/s3"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/sources/aws/sqs"
)

// EnsureQueue ensures the existence of a SQS queue for sending S3 event
// notifications.
func EnsureQueue(ctx context.Context, src *v1alpha1.AWSS3Source, cli sqsiface.SQSAPI) (string /*arn*/, error) {

	status := &src.Status

	if dest := src.Spec.Destination; dest != nil {
		if userProvidedQueue := dest.SQS; userProvidedQueue != nil {
			status.QueueARN = &userProvidedQueue.QueueARN
			return userProvidedQueue.QueueARN.String(), nil
		}
	}

	queueName := queueName(src)

	queueURL, err := sqs.QueueURL(cli, queueName)
	switch {
	case isNotFound(err):
		queueURL, err = sqs.CreateQueue(cli, queueName, queueTags(src))
		if err != nil {
			return "", fmt.Errorf("error creating SQS queue for event notifications: %w", toErrMsg(err))
		}

	case isAWSError(err):
		// All documented API errors require some user intervention and
		// are not to be retried.
		// https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
		return "", fmt.Errorf("request to SQS API got rejected: %w", toErrMsg(err))

	case err != nil:
		return "", fmt.Errorf("failed to determine URL of SQS queue: %w", toErrMsg(err))
	}

	getAttrs := []string{awssqs.QueueAttributeNameQueueArn, awssqs.QueueAttributeNamePolicy}
	queueAttrs, err := sqs.QueueAttributes(cli, queueURL, getAttrs)
	if err != nil {
		return "", fmt.Errorf("getting attributes of SQS queue: %w", err)
	}

	queueARN := queueAttrs[awssqs.QueueAttributeNameQueueArn]

	queueARNStruct, err := arnStrToARN(queueARN)
	if err != nil {
		return "", fmt.Errorf("converting ARN string to structured ARN: %w", err)
	}

	// it is essential that we propagate the queue's ARN here,
	// otherwise BuildAdapter() won't be able to configure the SQS
	// adapter properly
	status.QueueARN = queueARNStruct

	currentPol := unmarshalQueuePolicy(queueAttrs[awssqs.QueueAttributeNamePolicy])
	desiredPol := makeQueuePolicy(queueARN, src)

	if err := syncQueuePolicy(cli, queueURL, currentPol, desiredPol); err != nil {
		return "", fmt.Errorf("error synchronizing policy of SQS queue: %w", err)
	}

	return queueARN, nil
}

// EnsureNoQueue ensures that the SQS queue created for sending S3 event
// notifications is deleted.
func EnsureNoQueue(ctx context.Context, src *v1alpha1.AWSS3Source, cli sqsiface.SQSAPI) error {
	if dest := src.Spec.Destination; dest != nil {
		if userProvidedQueue := dest.SQS; userProvidedQueue != nil {
			// do not delete queues managed by the user
			return nil
		}
	}

	queueURL, err := sqs.QueueURL(cli, queueName(src))
	switch {
	case isNotFound(err):
		// event.Warn(ctx, ReasonUnsubscribed, "Queue not found, skipping deletion")
		return nil
	case isDenied(err):
		// it is unlikely that we recover from auth errors in the
		// finalizer, so we simply record a warning event and return
		// event.Warn(ctx, ReasonFailedUnsubscribe,
		// 	"Authorization error getting SQS queue. Ignoring: %s", toErrMsg(err))
		return nil
	case err != nil:
		return fmt.Errorf("failed to determine URL of SQS queue: %w", toErrMsg(err))
	}

	owns, err := assertOwnership(cli, queueURL, src)
	if err != nil {
		return fmt.Errorf("failed to verify owner of SQS queue: %w", toErrMsg(err))
		// return reconciler.NewEvent(corev1.EventTypeWarning, ReasonFailedUnsubscribe,
		// 	"Failed to verify owner of SQS queue: %s", toErrMsg(err))
	}

	if !owns {
		// event.Warn(ctx, ReasonUnsubscribed, "Queue %q is not owned by this source instance, "+
		// 	"skipping deletion", queueURL)
		return nil
	}

	err = sqs.DeleteQueue(cli, queueURL)
	switch {
	case isDenied(err):
		// it is unlikely that we recover from auth errors in the
		// finalizer, so we simply record a warning event and return
		// event.Warn(ctx, ReasonFailedUnsubscribe,
		// 	"Authorization error deleting SQS queue. Ignoring: %s", toErrMsg(err))
		return nil
	case err != nil:
		return fmt.Errorf("error deleting SQS queue: %w", toErrMsg(err))
		// return reconciler.NewEvent(corev1.EventTypeWarning, ReasonFailedUnsubscribe,
		// 	"Error deleting SQS queue: %s", toErrMsg(err))
	}

	// event.Normal(ctx, ReasonQueueDeleted, "Deleted SQS queue %q", queueURL)

	return nil
}

// syncQueuePolicy ensures that a SQS queue has the right permissions to
// receive messages from the S3 bucket observed by the given source.
func syncQueuePolicy(cli sqsiface.SQSAPI, queueURL string, current, desired iam.Policy) error {
	if equalPolicies(desired, current) {
		return nil
	}

	if err := sqs.SetQueuePolicy(cli, queueURL, desired); err != nil {
		return fmt.Errorf("setting policy of SQS queue: %w", err)
	}

	return nil
}

// equalPolicies returns whether two SQS policies are semantically equal.
func equalPolicies(a, b iam.Policy) bool {
	if len(a.Statement) != len(b.Statement) {
		return false
	}

	as, bs := a.Statement[0], b.Statement[0]

	if !reflect.DeepEqual(as.Principal, bs.Principal) {
		return false
	}
	if !reflect.DeepEqual(as.Condition, bs.Condition) {
		return false
	}
	if !reflect.DeepEqual(as.Action, bs.Action) {
		return false
	}
	return reflect.DeepEqual(as.Resource, bs.Resource)
}

// makeQueuePolicy creates an IAM policy for the given SQS queue ARN and source instance.
func makeQueuePolicy(queueARN string, src *v1alpha1.AWSS3Source) iam.Policy {
	bucketARN := s3.RealBucketARN(src.Spec.ARN)
	accID := src.Spec.ARN.AccountID

	return iam.NewPolicy(
		newS3ToSQSPolicyStatement(queueARN, bucketARN, accID),
	)
}

// newS3ToSQSPolicyStatement returns an IAM Policy Statement that allows a S3
// bucket to publish event notifications to the given SQS queue.
// Ref. https://docs.aws.amazon.com/AmazonS3/latest/userguide/grant-destinations-permissions-to-s3.html#grant-sns-sqs-permission-for-s3
func newS3ToSQSPolicyStatement(queueARN, bucketARN, accID string) iam.PolicyStatement {
	return iam.NewPolicyStatement(iam.EffectAllow,
		iam.PrincipalService("s3.amazonaws.com"),
		iam.ConditionArnEquals("aws:SourceArn", bucketARN),
		iam.ConditionStringEquals("aws:SourceAccount", accID),
		iam.Action("sqs:SendMessage"),
		iam.Resource(queueARN),
	)
}

// unmarshalQueuePolicy deserializes an IAM policy string.
func unmarshalQueuePolicy(polStr string) iam.Policy {
	var pol iam.Policy
	_ = json.Unmarshal([]byte(polStr), &pol)

	// if an error occured, the policy will be empty syncQueuePolicy() will
	// simply enforce the desired state
	return pol
}

// queueName returns a SQS queue name matching the given source instance.
func queueName(src *v1alpha1.AWSS3Source) string {
	return "s3-events_" + src.Spec.ARN.Resource
}

// assertOwnership returns whether a SQS queue identified by URL is owned by
// the given source.
func assertOwnership(cli sqsiface.SQSAPI, queueURL string, src *v1alpha1.AWSS3Source) (bool, error) {
	tags, err := sqs.QueueTags(cli, queueURL)
	if err != nil {
		return false, fmt.Errorf("listing tags of SQS queue: %w", err)
	}

	return tags["owned-by"] == sourceID(src), nil
}

// queueTags returns a set of tags containing information from the given source
// instance to set on a SQS queue.
func queueTags(src *v1alpha1.AWSS3Source) map[string]string {
	return map[string]string{
		"bucket-arn":    s3.RealBucketARN(src.Spec.ARN),
		"bucket-region": src.Spec.ARN.Region,
		"owned-by":      sourceID(src),
	}
}

// arnStrToARN returns the given ARN string as a structured ARN.
func arnStrToARN(arnStr string) (*apis.ARN, error) {
	arn, err := arn.Parse(arnStr)
	if err != nil {
		return nil, fmt.Errorf("parsing ARN string: %w", err)
	}

	apiARN := apis.ARN(arn)
	return &apiARN, nil
}

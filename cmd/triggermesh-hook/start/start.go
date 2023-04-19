// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package start

import (
	commoncmd "github.com/triggermesh/scoby-hook-triggermesh/pkg/common/cmd"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/handler"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/handler/kuards"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/sources/client/s3"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/sources/reconciler/awss3source"

	"github.com/triggermesh/scoby-hook-triggermesh/pkg/server"
)

type Cmd struct {
	Address string `help:"Address to listen for incoming requests." env:"ADDRESS" default:":8080"`
	Path    string `help:"Path where hook requests are served." env:"PATH" default:"v1"`
}

func (c *Cmd) Run(g *commoncmd.Globals) error {
	g.Logger.Debug("Creating TriggerMesh hook server")

	r := handler.NewRegistry([]handler.Handler{
		// Kuards is a temporary playground
		kuards.New(),
		awss3source.New(s3.NewClientGetter(g.KubeClient.CoreV1().Secrets), g.Logger),
	})

	s := server.New(c.Path, c.Address, r, g.DynClient, g.Logger)
	return s.Start(g.Context)
}

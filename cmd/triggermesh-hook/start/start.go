// Copyright 2022 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package start

import (
	commoncmd "github.com/triggermesh/scoby-hook-triggermesh/pkg/common/cmd"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/server"
)

type Cmd struct {
	Address string `help:"Address to listen for incoming requests." env:"ADDRESS" default:":8080"`
	Path    string `help:"Path where hook requests are served." env:"PATH" default:"v1"`
}

func (c *Cmd) Run(globals *commoncmd.Globals) error {
	globals.Logger.Debug("Creating TriggerMesh hook server")

	s := server.New(c.Path, c.Address, globals.Logger)
	return s.Start(globals.Context)
}

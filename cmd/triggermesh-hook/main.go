package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/triggermesh/scoby-hook-triggermesh/cmd/triggermesh-hook/start"
	commoncmd "github.com/triggermesh/scoby-hook-triggermesh/pkg/common/cmd"
)

type cli struct {
	commoncmd.Globals

	Start start.Cmd `cmd:"" help:"Starts the TriggerMesh hook for Scoby."`
}

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, os.Kill)
	defer stop()

	cli := cli{
		Globals: commoncmd.Globals{
			Context: ctx,
		},
	}

	kc := kong.Parse(&cli)

	err := cli.Initialize()
	if err != nil {
		panic(fmt.Errorf("error initializing: %w", err))
	}
	defer cli.Flush()

	err = kc.Run(&cli.Globals)
	kc.FatalIfErrorf(err)
}

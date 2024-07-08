package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/synternet/data-layer-sdk/pkg/network"
	"github.com/synternet/data-layer-sdk/pkg/options"
	"github.com/synternet/data-layer-sdk/pkg/service"
	"github.com/synternet/data-layer-sdk/pkg/user"

	svc "github.com/synternet/ethereum-publisher/internal/service"
	"github.com/synternet/ethereum-publisher/pkg/ipc"
)

func setDefault(field string, value string) {
	if os.Getenv(field) == "" {
		os.Setenv(field, value)
	}
}

func main() {
	const (
		NATS_URLS        = "NATS_URLS"
		IPC_PATH         = "IPC_PATH"
		NATS_ACC_NKEY    = "NATS_ACC_NKEY"
		NATS_NKEY        = "NATS_NKEY"
		NATS_JWT         = "NATS_JWT"
		PUBLISHER_PREFIX = "PUBLISHER_PREFIX"
		PUBLISHER_NAME   = "PUBLISHER_NAME"
	)

	setDefault(PUBLISHER_NAME, "ethereum")

	flagIpcPath := flag.String("ipc-path", os.Getenv(IPC_PATH), "Blockchain node IPC path")
	flagNatsUrls := flag.String("nats-urls", os.Getenv(NATS_URLS), "Broker NATS urls (comma separated)")
	// We do not document this not to promise this feature in future iterations.
	flagNatsAccNkey := flag.String("nats-acc-nkey", os.Getenv(NATS_ACC_NKEY), "NATS account NKEY")
	flagNatsNkey := flag.String("nats-nkey", os.Getenv(NATS_NKEY), "NATS user NKEY")
	flagNatsJwt := flag.String("nats-jwt", os.Getenv(NATS_JWT), "NATS user JWT")
	flagPublisherPrefix := flag.String("publisher-prefix", os.Getenv(PUBLISHER_PREFIX), "Publisher prefix ({prefix}.{name}.<..>)")
	flagPublisherName := flag.String("publisher-name", os.Getenv(PUBLISHER_NAME), "Publisher name ({prefix}.{name}.<..>)")
	flag.Parse()

	if *flagIpcPath == "" {
		log.Fatal("missing IPC path")
	}

	// Sacrifice some security for the sake of user experience by allowing to
	// supply NATS account NKey instead of passing created user NKey and user JWS.
	if *flagNatsAccNkey != "" {
		nkey, jwt, err := user.CreateCreds([]byte(*flagNatsAccNkey))
		flagNatsNkey = &nkey
		flagNatsJwt = &jwt

		if err != nil {
			panic(fmt.Errorf("failed to generate user JWT: %w", err))
		}
	}

	network.SetDefault("testnet")

	conn, err := options.MakeNats("Base publisher (NATS connection)", *flagNatsUrls, "", *flagNatsNkey, *flagNatsJwt, "", "", "")
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ipcSvc, errSvc := ipc.NewClient(ctx, *flagIpcPath)
	if errSvc != nil {
		log.Fatalf("Starting service error: %s", errSvc.Error())
	}
	log.Println("IPC socket connected.")

	pub := svc.New(ipcSvc, service.WithContext(ctx), service.WithPrefix(*flagPublisherPrefix), service.WithName(*flagPublisherName), service.WithNats(conn))

	pubCtx := pub.Start()

	defer pub.Close()

	select {
	case <-ctx.Done():
		fmt.Println("Shutdown")
	case <-pubCtx.Done():
		fmt.Println("Publisher stopped", "cause", context.Cause(pubCtx).Error())
	}
}

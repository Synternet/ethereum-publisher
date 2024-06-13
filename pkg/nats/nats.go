package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type NatsService struct {
	nats *natsConnection
}

type Handler func([]byte) error

type Config struct {
	URI  string
	Opts []nats.Option
}

type natsConnection struct {
	conn          *nats.Conn
	subscriptions []*nats.Subscription
	handlers      map[string]Handler
	errorCh       <-chan error
}

func MustConnect(cfg Config) *NatsService {
	nats, err := Connect(cfg)
	if err != nil {
		panic(fmt.Errorf("failed to connect nats: %w", err))
	}
	return nats
}

func Connect(cfg Config) (*NatsService, error) {
	conn, errorCh, err := connect(cfg)
	if err != nil {
		return &NatsService{}, err
	}

	s := newService(conn, errorCh)

	return s, nil
}

func connect(cfg Config) (*nats.Conn, <-chan error, error) {
	errorCh := make(chan error, 1)

	totalWait := 10 * time.Minute
	reconnectDelay := time.Second

	cfg.Opts = append(cfg.Opts, nats.ReconnectWait(reconnectDelay))
	cfg.Opts = append(cfg.Opts, nats.MaxReconnects(int(totalWait/reconnectDelay)))
	cfg.Opts = append(cfg.Opts, nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
		reason := fmt.Errorf("disconnected due to:%s, will attempt reconnects for %.0fm", err, totalWait.Minutes())
		errorCh <- reason
	}))
	cfg.Opts = append(cfg.Opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		reason := fmt.Errorf("reconnected [%s]", nc.ConnectedUrl())
		errorCh <- reason
	}))
	cfg.Opts = append(cfg.Opts, nats.ClosedHandler(func(nc *nats.Conn) {
		reason := fmt.Errorf("exiting: %v", nc.LastError())
		errorCh <- reason
	}))

	natsConn, err := nats.Connect(cfg.URI, cfg.Opts...)
	if err != nil {
		errorCh <- err
		return natsConn, errorCh, err
	}
	return natsConn, errorCh, err
}

// newService creates nats Service, introduced for testing purpose
func newService(conn *nats.Conn, errorCh <-chan error) *NatsService {
	return &NatsService{
		nats: &natsConnection{
			conn:     conn,
			handlers: map[string]Handler{},
			errorCh:  errorCh,
		},
	}
}

func (sn *NatsService) AddHandler(subject string, handlerFn Handler) {
	if _, ok := sn.nats.handlers[subject]; ok {
		panic(fmt.Errorf("handler with subject %s already registered", subject))
	}

	sn.nats.handlers[subject] = handlerFn
}

func (sn *NatsService) CloseConnection() {
	sn.nats.conn.Flush()
	sn.nats.conn.Close()
}

func (sn *NatsService) Serve(ctx context.Context) (err error) {
	if err = sn.subscribeHandlers(ctx); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-sn.nats.errorCh:
	}
	return err
}

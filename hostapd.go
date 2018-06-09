package main

import (
	"context"
	"flag"
	"time"

	"github.com/pkg/errors"
	"go.jonnrb.io/hostapd_grpc/proto"
	"google.golang.org/grpc"
)

var (
	serverAddr = flag.String("hostapd.addr", "", "The address of the hostapd control grpc server")
)

type HostapdControl struct {
	conn   *grpc.ClientConn
	client hostapd.HostapdControlClient
}

func NewHostapdControl(ctx context.Context) (*HostapdControl, error) {
	var opts []grpc.DialOption

	// TODO: add TLS options
	opts = append(opts, grpc.WithInsecure())

	opts = append(opts, grpc.WithBackoffMaxDelay(10*time.Second))

	conn, err := grpc.DialContext(ctx, *serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	client := hostapd.NewHostapdControlClient(conn)

	return &HostapdControl{conn, client}, err
}

func (h *HostapdControl) Close() error {
	return h.conn.Close()
}

func (h *HostapdControl) ListClients(ctx context.Context) ([]*hostapd.Client, error) {
	res, err := h.client.ListClients(ctx, &hostapd.ListClientsRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "error listing hostapd clients")
	}
	return res.Client, nil
}

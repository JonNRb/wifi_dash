package main

import (
	"context"
	"flag"
	"time"

	pb "github.com/jonnrb/hostapd_grpc/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

var (
	serverAddr = flag.String("hostapd.addr", "", "The address of the hostapd control grpc server")
)

type HostapdControl struct {
	conn   *grpc.ClientConn
	client pb.HostapdControlClient
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

	client := pb.NewHostapdControlClient(conn)

	return &HostapdControl{conn, client}, err
}

func (h *HostapdControl) Close() error {
	return h.conn.Close()
}

func (h *HostapdControl) ListClients(ctx context.Context) ([]*pb.Client, error) {
	res, err := h.client.ListClients(ctx, &pb.ListRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "error listing hostapd clients")
	}
	return res.Client, nil
}

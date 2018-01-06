package main

import (
	"context"
	"errors"
	"flag"
	"net"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	etcdhcp "github.com/jonnrb/etcdhcp/proto"
)

type CommaSeparated []string

var (
	autoSyncInterval = flag.Duration("etcd.auto_sync_interval", 60*time.Second, "How often to sync etcd endpoints")
	serverAddrs      = CommaSeparated{}
	prefixes         = CommaSeparated{"etcdhcp::"}
)

func init() {
	flag.Var(&serverAddrs, "etcd.addrs", "The set of etcd endpoints (separated by commas) that host etcdhcp leases")
	flag.Var(&prefixes, "etcd.dhcp_prefixes", "The prefixes (separated by commas) that store etcdhcp leases")
}

func (c *CommaSeparated) Set(val string) error {
	*c = strings.Split(val, ",")
	return nil
}

func (c *CommaSeparated) String() string {
	return strings.Join(*c, ",")
}

type DHCPStore struct {
	c *etcd.Client
}

func NewDHCPStore() (*DHCPStore, error) {
	if len(serverAddrs) == 0 {
		return nil, errors.New("at least one etcd endpoint must be provided with -etcd.addrs")
	}

	if len(prefixes) == 0 {
		return nil, errors.New("at least one etcdhcp prefix must be provided with -etcd.dhcp_prefixes")
	}

	c, err := etcd.New(etcd.Config{
		Endpoints:        serverAddrs,
		AutoSyncInterval: *autoSyncInterval,
		DialTimeout:      5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &DHCPStore{
		c: c,
	}, nil
}

func (d *DHCPStore) Close() error {
	return d.c.Close()
}

func (d *DHCPStore) LookupDevice(ctx context.Context, nic net.HardwareAddr) ([]net.IP, *etcdhcp.ClientInfo, error) {
	ipsC := make(chan net.IP)
	ciC := make(chan *etcdhcp.ClientInfo)

	for _, p := range prefixes {
		go func(prefix string) {
			var ip net.IP
			key := prefix + "nics::leased::" + nic.String()
			res, err := d.c.KV.Get(ctx, key)
			if err != nil {
				glog.Errorf("unable to get key \"%v\" from etcd: %v", key, err)
			} else {
				switch len(res.Kvs) {
				case 0:
				case 1:
					ip = net.ParseIP(string(res.Kvs[0].Value))
					if ip == nil {
						glog.Errorf("unable to parse ip %v", res.Kvs[0].Value)
					}
				default:
					glog.Errorf("invalid response from etcd on request for single key \"%v\"", key)
				}
			}
			ipsC <- ip
		}(p)
		go func(prefix string) {
			var ci *etcdhcp.ClientInfo
			key := prefix + "nics::info::" + nic.String()
			res, err := d.c.KV.Get(ctx, key)
			if err != nil {
				glog.Errorf("unable to get key \"%v\" from etcd: %v", key, err)
			} else {
				switch len(res.Kvs) {
				case 0:
				case 1:
					ci = &etcdhcp.ClientInfo{}
					err = proto.Unmarshal(res.Kvs[0].Value, ci)
					if err != nil {
						glog.Error("couldn't unpack client info proto: %v", err)
					}
				default:
					glog.Errorf("invalid response from etcd on request for single key \"%v\"", key)
				}
			}
			ciC <- ci
		}(p)
	}

	var ci *etcdhcp.ClientInfo
	ips := []net.IP{}
	for i := 0; i < 2*len(prefixes); i++ {
		select {
		case ip := <-ipsC:
			if ip != nil {
				ips = append(ips, ip)
			}
		case ciCand := <-ciC:
			if ci == nil {
				ci = ciCand
			} else if ciCand == nil {
			} else if *ci != *ciCand {
				// This could be pretty darn interesting. (Eh, maybe.)
				glog.V(2).Infof("one client info (%v) didn't match another (%v)", *ci, *ciCand)
			}
		}
	}

	// This may be `nil` and I don't like returning `nil` as a value on success
	if ci == nil {
		ci = &etcdhcp.ClientInfo{}
	}

	return ips, ci, nil
}

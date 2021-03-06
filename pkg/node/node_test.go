// Copyright 2016-2019 Authors of Cilium
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !privileged_tests

package node

import (
	"net"
	"testing"

	"github.com/cilium/cilium/pkg/checker"
	"github.com/cilium/cilium/pkg/cidr"
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/node/addressing"
	"github.com/cilium/cilium/pkg/source"

	. "gopkg.in/check.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	TestingT(t)
}

type NodeSuite struct{}

var _ = Suite(&NodeSuite{})

func (s *NodeSuite) TestGetNodeIP(c *C) {
	n := Node{
		Name: "node-1",
		IPAddresses: []Address{
			{IP: net.ParseIP("192.0.2.3"), Type: addressing.NodeExternalIP},
		},
	}
	ip := n.GetNodeIP(false)
	// Return the only IP present
	c.Assert(ip.Equal(net.ParseIP("192.0.2.3")), Equals, true)

	n.IPAddresses = append(n.IPAddresses, Address{IP: net.ParseIP("192.0.2.3"), Type: addressing.NodeExternalIP})
	ip = n.GetNodeIP(false)
	// The next priority should be NodeExternalIP
	c.Assert(ip.Equal(net.ParseIP("192.0.2.3")), Equals, true)

	n.IPAddresses = append(n.IPAddresses, Address{IP: net.ParseIP("198.51.100.2"), Type: addressing.NodeInternalIP})
	ip = n.GetNodeIP(false)
	// The next priority should be NodeInternalIP
	c.Assert(ip.Equal(net.ParseIP("198.51.100.2")), Equals, true)

	n.IPAddresses = append(n.IPAddresses, Address{IP: net.ParseIP("2001:DB8::1"), Type: addressing.NodeExternalIP})
	ip = n.GetNodeIP(true)
	// The next priority should be NodeExternalIP and IPv6
	c.Assert(ip.Equal(net.ParseIP("2001:DB8::1")), Equals, true)

	n.IPAddresses = append(n.IPAddresses, Address{IP: net.ParseIP("2001:DB8::2"), Type: addressing.NodeInternalIP})
	ip = n.GetNodeIP(true)
	// The next priority should be NodeInternalIP and IPv6
	c.Assert(ip.Equal(net.ParseIP("2001:DB8::2")), Equals, true)

	n.IPAddresses = append(n.IPAddresses, Address{IP: net.ParseIP("198.51.100.2"), Type: addressing.NodeInternalIP})
	ip = n.GetNodeIP(false)
	// Should still return NodeInternalIP and IPv4
	c.Assert(ip.Equal(net.ParseIP("198.51.100.2")), Equals, true)

}

func (s *NodeSuite) TestParseCiliumNode(c *C) {
	nodeResource := &ciliumv2.CiliumNode{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: ciliumv2.NodeSpec{
			Addresses: []ciliumv2.NodeAddress{
				{Type: addressing.NodeInternalIP, IP: "2.2.2.2"},
				{Type: addressing.NodeExternalIP, IP: "3.3.3.3"},
				{Type: addressing.NodeInternalIP, IP: "c0de::1"},
				{Type: addressing.NodeExternalIP, IP: "c0de::2"},
			},
			Encryption: ciliumv2.EncryptionSpec{
				Key: 10,
			},
			IPAM: ciliumv2.IPAMSpec{
				PodCIDRs: []string{
					"10.10.0.0/16",
					"c0de::/96",
				},
			},
			HealthAddressing: ciliumv2.HealthAddressingSpec{
				IPv4: "1.1.1.1",
				IPv6: "c0de::1",
			},
		},
	}

	n := ParseCiliumNode(nodeResource)
	c.Assert(n, checker.DeepEquals, Node{
		Name:   "foo",
		Source: source.CustomResource,
		IPAddresses: []Address{
			{Type: addressing.NodeInternalIP, IP: net.ParseIP("2.2.2.2")},
			{Type: addressing.NodeExternalIP, IP: net.ParseIP("3.3.3.3")},
			{Type: addressing.NodeInternalIP, IP: net.ParseIP("c0de::1")},
			{Type: addressing.NodeExternalIP, IP: net.ParseIP("c0de::2")},
		},
		EncryptionKey: uint8(10),
		IPv4AllocCIDR: cidr.MustParseCIDR("10.10.0.0/16"),
		IPv6AllocCIDR: cidr.MustParseCIDR("c0de::/96"),
		IPv4HealthIP:  net.ParseIP("1.1.1.1"),
		IPv6HealthIP:  net.ParseIP("c0de::1"),
	})
}

func (s *NodeSuite) TestPublicAttrEquals(c *C) {
	type fields struct {
		Name          string
		Cluster       string
		IPAddresses   []Address
		IPv4AllocCIDR *cidr.CIDR
		IPv6AllocCIDR *cidr.CIDR
		IPv4HealthIP  net.IP
		IPv6HealthIP  net.IP
		ClusterID     int
		Source        source.Source
	}
	type args struct {
		o *Node
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "test nil equalness",
			fields: fields{},
			args:   args{o: nil},
			want:   false,
		},
		{
			name: "test equalness",
			fields: fields{
				Name:          "foo",
				Cluster:       "cluster-1",
				IPv4HealthIP:  net.ParseIP("1.1.1.1"),
				IPv6HealthIP:  net.ParseIP("fd00::1"),
				ClusterID:     1,
				Source:        source.Kubernetes,
				IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.1"), Type: addressing.NodeHostName}},
				IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
				IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
			},
			args: args{
				o: &Node{
					Name:          "foo",
					Cluster:       "cluster-1",
					IPv4HealthIP:  net.ParseIP("1.1.1.1"),
					IPv6HealthIP:  net.ParseIP("fd00::1"),
					ClusterID:     1,
					Source:        source.Kubernetes,
					IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.1"), Type: addressing.NodeHostName}},
					IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
					IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
				},
			},
			want: true,
		},
		{
			name: "test different IPAddresses length",
			fields: fields{
				Name:          "foo",
				Cluster:       "cluster-1",
				IPv4HealthIP:  net.ParseIP("1.1.1.1"),
				IPv6HealthIP:  net.ParseIP("fd00::1"),
				ClusterID:     1,
				Source:        source.Kubernetes,
				IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.1"), Type: addressing.NodeHostName}},
				IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
				IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
			},
			args: args{
				o: &Node{
					Name:          "foo",
					Cluster:       "cluster-1",
					IPv4HealthIP:  net.ParseIP("1.1.1.1"),
					IPv6HealthIP:  net.ParseIP("fd00::1"),
					ClusterID:     1,
					Source:        source.Kubernetes,
					IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
					IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
				},
			},
			want: false,
		},
		{
			name: "test different IPv4AllocCIDR",
			fields: fields{
				Name:          "foo",
				Cluster:       "cluster-1",
				IPv4HealthIP:  net.ParseIP("1.1.1.1"),
				IPv6HealthIP:  net.ParseIP("fd00::1"),
				ClusterID:     1,
				Source:        source.Kubernetes,
				IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.1"), Type: addressing.NodeHostName}},
				IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
				IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
			},
			args: args{
				o: &Node{
					Name:          "foo",
					Cluster:       "cluster-1",
					IPv4HealthIP:  net.ParseIP("1.1.1.1"),
					IPv6HealthIP:  net.ParseIP("fd00::1"),
					ClusterID:     1,
					Source:        source.Kubernetes,
					IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.0"), Type: addressing.NodeHostName}},
					IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
				},
			},
			want: false,
		},
		{
			name: "test different IPv6AllocCIDR",
			fields: fields{
				Name:          "foo",
				Cluster:       "cluster-1",
				IPv4HealthIP:  net.ParseIP("1.1.1.1"),
				IPv6HealthIP:  net.ParseIP("fd00::1"),
				ClusterID:     1,
				Source:        source.Kubernetes,
				IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.1"), Type: addressing.NodeHostName}},
				IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
				IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
			},
			args: args{
				o: &Node{
					Name:          "foo",
					Cluster:       "cluster-1",
					IPv4HealthIP:  net.ParseIP("1.1.1.1"),
					IPv6HealthIP:  net.ParseIP("fd00::1"),
					ClusterID:     1,
					Source:        source.Kubernetes,
					IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.0"), Type: addressing.NodeHostName}},
					IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
				},
			},
			want: false,
		},
		{
			name: "test different name",
			fields: fields{
				Name:          "foo",
				Cluster:       "cluster-1",
				IPv4HealthIP:  net.ParseIP("1.1.1.1"),
				IPv6HealthIP:  net.ParseIP("fd00::1"),
				ClusterID:     1,
				Source:        source.Kubernetes,
				IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.1"), Type: addressing.NodeHostName}},
				IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
				IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
			},
			args: args{
				o: &Node{
					Cluster:       "cluster-1",
					IPv4HealthIP:  net.ParseIP("1.1.1.1"),
					IPv6HealthIP:  net.ParseIP("fd00::1"),
					ClusterID:     1,
					Source:        source.Kubernetes,
					IPAddresses:   []Address{{IP: net.ParseIP("1.1.1.0"), Type: addressing.NodeHostName}},
					IPv4AllocCIDR: cidr.MustParseCIDR("1.1.1.1/24"),
					IPv6AllocCIDR: cidr.MustParseCIDR("fd00::1/64"),
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		n := &Node{
			Name:          tt.fields.Name,
			Cluster:       tt.fields.Cluster,
			IPAddresses:   tt.fields.IPAddresses,
			IPv4AllocCIDR: tt.fields.IPv4AllocCIDR,
			IPv6AllocCIDR: tt.fields.IPv6AllocCIDR,
			IPv4HealthIP:  tt.fields.IPv4HealthIP,
			IPv6HealthIP:  tt.fields.IPv6HealthIP,
			ClusterID:     tt.fields.ClusterID,
			Source:        tt.fields.Source,
		}
		c.Logf(tt.name)
		got := n.PublicAttrEquals(tt.args.o)
		c.Assert(got, Equals, tt.want)
	}
}

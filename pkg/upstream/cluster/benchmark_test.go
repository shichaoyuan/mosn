/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cluster

import (
	"testing"

	v2 "mosn.io/mosn/pkg/api/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

func TestMain(m *testing.M) {
	log.DefaultLogger.SetLogLevel(log.ERROR)
	m.Run()
}

// BenchmarkHostSetRefresh test host heatlthy state changed
func BenchmarkHostSetRefresh(b *testing.B) {
	subsetKeys := [][]string{
		[]string{"zone", "version"},
		[]string{"zone"},
	}
	createHostSet := func(m map[int]v2.Metadata) *hostSet {
		count := 0
		for cnt := range m {
			count += cnt
		}
		pool := makePool(count)
		totalHosts := make([]types.Host, 0, count)
		for cnt, meta := range m {
			totalHosts = append(totalHosts, pool.MakeHosts(cnt, meta)...)
		}
		hs := &hostSet{}
		hs.setFinalHost(totalHosts)
		for _, meta := range m {
			for _, keys := range subsetKeys {
				kvs := ExtractSubsetMetadata(keys, meta)
				hs.createSubset(func(h types.Host) bool {
					return HostMatches(kvs, h)
				})
			}
		}
		return hs
	}
	b.Run("RefreshSimple100", func(b *testing.B) {
		config := map[int]v2.Metadata{
			100: nil,
		}
		hs := createHostSet(config)
		host := hs.Hosts()[50]
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				host.SetHealthFlag(types.FAILED_ACTIVE_HC)
			} else {
				host.ClearHealthFlag(types.FAILED_ACTIVE_HC)
			}
			hs.refreshHealthHost(host)
		}
	})
	b.Run("RefreshSimple1000", func(b *testing.B) {
		config := map[int]v2.Metadata{
			1000: nil,
		}
		hs := createHostSet(config)
		host := hs.Hosts()[500]
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				host.SetHealthFlag(types.FAILED_ACTIVE_HC)
			} else {
				host.ClearHealthFlag(types.FAILED_ACTIVE_HC)
			}
			hs.refreshHealthHost(host)
		}
	})
	b.Run("RefreshMeta1000", func(b *testing.B) {
		config := map[int]v2.Metadata{
			100: nil,
			300: v2.Metadata{
				"zone":    "a",
				"version": "1.0",
			},
			400: v2.Metadata{
				"zone":    "a",
				"version": "2.0",
			},
			200: v2.Metadata{
				"zone": "b",
			},
		}
		hs := createHostSet(config)
		host := hs.Hosts()[150] // zone:a, version:1.0
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				host.SetHealthFlag(types.FAILED_ACTIVE_HC)
			} else {
				host.ClearHealthFlag(types.FAILED_ACTIVE_HC)
			}
			hs.refreshHealthHost(host)
		}
	})
}

func BenchmarkHostConfig(b *testing.B) {
	host := &simpleHost{
		hostname:      "Testhost",
		addressString: "127.0.0.1:8080",
		weight:        100,
		metaData: v2.Metadata{
			"zone":    "a",
			"version": "1",
		},
	}
	b.Run("Host.Config", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			host.Config()
		}
	})
}

func BenchmarkUpdateClusterHosts(b *testing.B) {
	cluster := _createTestCluster()
	// assume cluster have 1000 hosts
	pool := makePool(1010)
	hosts := make([]types.Host, 0, 1000)
	metas := []v2.Metadata{
		v2.Metadata{"version": "1", "zone": "a"},
		v2.Metadata{"version": "1", "zone": "b"},
		v2.Metadata{"version": "2", "zone": "a"},
	}
	for _, meta := range metas {
		hosts = append(hosts, pool.MakeHosts(300, meta)...)
	}
	noLabelHosts := pool.MakeHosts(100, nil)
	hosts = append(hosts, noLabelHosts...)
	// hosts changes, some are removed, some are added
	var newHosts []types.Host
	for idx := range metas {
		newHosts = append(newHosts, hosts[idx:idx*300+5]...)
	}
	newHosts = append(newHosts, pool.MakeHosts(10, v2.Metadata{
		"version": "3",
		"zone":    "b",
	})...)
	newHosts = append(newHosts, noLabelHosts...)
	b.Run("UpdateClusterHost", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			cluster.UpdateHosts(hosts)
			b.StartTimer()
			cluster.UpdateHosts(newHosts)
		}
	})

}

func BencmarkUpdateClusterHostsLabel(b *testing.B) {
	cluster := _createTestCluster()
	pool := makePool(1000)
	hosts := make([]types.Host, 0, 1000)
	metas := []v2.Metadata{
		v2.Metadata{"zone": "a"},
		v2.Metadata{"zone": "b"},
	}
	for _, meta := range metas {
		hosts = append(hosts, pool.MakeHosts(500, meta)...)
	}
	newHosts := make([]types.Host, 1000)
	copy(newHosts, hosts)
	for i := 0; i < 500; i++ {
		host := newHosts[i]
		var ver string
		if i%2 == 0 {
			ver = "1.0"
		} else {
			ver = "2.0"
		}
		newHost := &mockHost{
			addr: host.AddressString(),
			meta: v2.Metadata{
				"version": ver,
				"zone":    "a",
			},
		}
		newHosts[i] = newHost
	}
	b.Run("UpdateClusterHostsLabel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			cluster.UpdateHosts(hosts)
			b.StartTimer()
			cluster.UpdateHosts(newHosts)
		}
	})
}

func BenchmarkClusterManagerRemoveHosts(b *testing.B) {
	_createClusterManager()
	pool := makePool(1200)
	hostsConfig := make([]v2.Host, 0, 1200)
	metas := []v2.Metadata{
		v2.Metadata{"version": "1", "zone": "a"},
		v2.Metadata{"version": "1", "zone": "b"},
		v2.Metadata{"version": "2", "zone": "a"},
		nil,
	}
	for _, meta := range metas {
		hosts := pool.MakeHosts(300, meta)
		for _, host := range hosts {
			hostsConfig = append(hostsConfig, v2.Host{
				HostConfig: v2.HostConfig{
					Address: host.AddressString(),
				},
				MetaData: meta,
			})
		}
	}
	var removeList []string
	for i := 0; i < 4; i++ {
		for j := 0; j < 10; j++ {
			host := hostsConfig[i*300+j]
			removeList = append(removeList, host.Address)
		}
	}
	b.Run("ClusterRemoveHosts", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			GetClusterMngAdapterInstance().UpdateClusterHosts("test1", hostsConfig)
			b.StartTimer()
			GetClusterMngAdapterInstance().RemoveClusterHosts("test1", removeList)
		}
	})

}

func BenchmarkClusterManagerAppendHosts(b *testing.B) {
	_createClusterManager()
	pool := makePool(1240)
	hostsConfig := make([]v2.Host, 0, 1200)
	metas := []v2.Metadata{
		v2.Metadata{"version": "1", "zone": "a"},
		v2.Metadata{"version": "1", "zone": "b"},
		v2.Metadata{"version": "2", "zone": "a"},
		nil,
	}
	for _, meta := range metas {
		hosts := pool.MakeHosts(300, meta)
		for _, host := range hosts {
			hostsConfig = append(hostsConfig, v2.Host{
				HostConfig: v2.HostConfig{
					Address: host.AddressString(),
				},
				MetaData: meta,
			})
		}
	}
	var addHostsCfg []v2.Host
	for _, meta := range metas {
		hs := pool.MakeHosts(10, nil)
		for _, h := range hs {
			addHostsCfg = append(addHostsCfg, v2.Host{
				HostConfig: v2.HostConfig{
					Address: h.AddressString(),
				},
				MetaData: meta,
			})
		}
	}
	b.Run("ClusterManagerAppendHosts", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			GetClusterMngAdapterInstance().UpdateClusterHosts("test1", hostsConfig)
			b.StartTimer()
			GetClusterMngAdapterInstance().AppendClusterHosts("test1", addHostsCfg)
		}
	})
}

func BenchmarkRandomLB(b *testing.B) {
	hostSet := &hostSet{}
	hosts := makePool(10).MakeHosts(10, nil)
	hostSet.setFinalHost(hosts)
	lb := newRandomLoadBalancer(hostSet)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lb.ChooseHost(nil)
		}
	})
}

func BenchmarkRoundRobinLB(b *testing.B) {
	hostSet := &hostSet{}
	hosts := makePool(10).MakeHosts(10, nil)
	hostSet.setFinalHost(hosts)
	lb := rrFactory.newRoundRobinLoadBalancer(hostSet)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lb.ChooseHost(nil)
		}
	})
}

func BenchmarkSubsetLB(b *testing.B) {
	hostSet := &hostSet{}
	subsetConfig := &v2.LBSubsetConfig{
		FallBackPolicy: 2,
		SubsetSelectors: [][]string{
			[]string{
				"zone",
			},
			[]string{
				"zone", "version",
			},
		},
	}
	pool := makePool(10)
	var hosts []types.Host
	hosts = append(hosts, pool.MakeHosts(5, v2.Metadata{
		"zone":    "RZ41A",
		"version": "1.0.0",
	})...)
	hosts = append(hosts, pool.MakeHosts(5, v2.Metadata{
		"zone":    "RZ41A",
		"version": "2.0.0",
	})...)
	hostSet.setFinalHost(hosts)
	lb := newSubsetLoadBalancer(types.Random, hostSet, newClusterStats("BenchmarkSubsetLB"), NewLBSubsetInfo(subsetConfig))
	b.Run("CtxZone", func(b *testing.B) {
		ctx := newMockLbContext(map[string]string{
			"zone": "RZ41A",
		})
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				lb.ChooseHost(ctx)
			}
		})
	})
	b.Run("CtxZoneAndVersion", func(b *testing.B) {
		ctx := newMockLbContext(map[string]string{
			"zone":    "RZ41A",
			"version": "1.0.0",
		})
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				lb.ChooseHost(ctx)
			}
		})
	})
	b.Run("CtxNotMatched", func(b *testing.B) {
		ctx := newMockLbContext(map[string]string{
			"version": "1.0.0",
		})
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				lb.ChooseHost(ctx)
			}
		})
	})
}

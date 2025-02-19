package cluster_test

import (
	"context"
	"github.com/rudderlabs/rudder-server/config"
	"github.com/rudderlabs/rudder-server/jobsdb"
	"github.com/rudderlabs/rudder-server/services/multitenant"
	"github.com/rudderlabs/rudder-server/services/stats"
	"github.com/rudderlabs/rudder-server/utils/logger"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudderlabs/rudder-server/app/cluster"
	"github.com/rudderlabs/rudder-server/utils/types/servermode"
	"github.com/stretchr/testify/require"
)

type mockModeProvider struct {
	ch chan servermode.Ack
}

func (m *mockModeProvider) ServerMode() (<-chan servermode.Ack, error) {
	return m.ch, nil
}

func (m *mockModeProvider) SendMode(newMode servermode.Ack) {
	m.ch <- newMode
}

func (m *mockModeProvider) Close() {
	close(m.ch)
}

type staticModeProvider servermode.Mode

func (s *staticModeProvider) ServerMode() <-chan servermode.Ack {
	ch := make(chan servermode.Ack, 1)
	ch <- servermode.WithACK(servermode.Mode(*s), func() {})
	close(ch)
	return ch
}

type mockLifecycle struct {
	status    string
	callOrder uint64
	callCount *uint64
}

func (m *mockLifecycle) Start() {
	m.callOrder = atomic.AddUint64(m.callCount, 1)
	m.status = "start"
}

func (m *mockLifecycle) Stop() {
	m.callOrder = atomic.AddUint64(m.callCount, 1)
	m.status = "stop"
}

func Init() {
	config.Load()
	stats.Setup()
	logger.Init()
}

func TestDynamicCluster(t *testing.T) {
	Init()

	provider := &mockModeProvider{ch: make(chan servermode.Ack)}

	callCount := uint64(0)

	gatewayDB := &mockLifecycle{status: "", callCount: &callCount}
	routerDB := &mockLifecycle{status: "", callCount: &callCount}
	batchRouterDB := &mockLifecycle{status: "", callCount: &callCount}
	errorDB := &mockLifecycle{status: "", callCount: &callCount}

	processor := &mockLifecycle{status: "", callCount: &callCount}
	router := &mockLifecycle{status: "", callCount: &callCount}

	mtStat := &multitenant.MultitenantStatsT{
		RouterDBs: map[string]jobsdb.MultiTenantJobsDB{},
	}
	dc := cluster.Dynamic{
		Provider: provider,

		GatewayDB:     gatewayDB,
		RouterDB:      routerDB,
		BatchRouterDB: batchRouterDB,
		ErrorDB:       errorDB,

		Processor: processor,
		Router:    router,

		MultiTenantStat: mtStat,
	}

	ctx, cancel := context.WithCancel(context.Background())

	wait := make(chan bool)
	go func() {
		dc.Run(ctx)
		close(wait)
	}()

	t.Run("DEGRADED -> NORMAL", func(t *testing.T) {
		chACK := make(chan bool)
		provider.SendMode(servermode.WithACK(servermode.NormalMode, func() {
			close(chACK)
		}))

		require.Eventually(t, func() bool {
			<-chACK
			return true
		}, time.Second, time.Millisecond)

		require.Equal(t, "start", gatewayDB.status)
		require.Equal(t, "start", routerDB.status)
		require.Equal(t, "start", batchRouterDB.status)
		require.Equal(t, "start", errorDB.status)

		require.Equal(t, "start", processor.status)
		require.Equal(t, "start", router.status)

		t.Log("dbs should be started before processor")
		require.True(t, gatewayDB.callOrder < processor.callOrder)
		require.True(t, routerDB.callOrder < processor.callOrder)
		require.True(t, batchRouterDB.callOrder < processor.callOrder)
		require.True(t, errorDB.callOrder < processor.callOrder)

		t.Log("dbs should be started before router")
		require.True(t, gatewayDB.callOrder < router.callOrder)
		require.True(t, routerDB.callOrder < router.callOrder)
		require.True(t, batchRouterDB.callOrder < router.callOrder)
		require.True(t, errorDB.callOrder < router.callOrder)
	})

	t.Run("NORMAL -> DEGRADED", func(t *testing.T) {
		chACK := make(chan bool)
		provider.SendMode(servermode.WithACK(servermode.DegradedMode, func() {
			close(chACK)
		}))

		require.Eventually(t, func() bool {
			<-chACK
			return true
		}, time.Second, time.Millisecond)

		require.Equal(t, "stop", gatewayDB.status)
		require.Equal(t, "stop", routerDB.status)
		require.Equal(t, "stop", batchRouterDB.status)
		require.Equal(t, "stop", errorDB.status)

		require.Equal(t, "stop", processor.status)
		require.Equal(t, "stop", router.status)

		t.Log("dbs should be stopped after processor")
		require.True(t, gatewayDB.callOrder > processor.callOrder)
		require.True(t, routerDB.callOrder > processor.callOrder)
		require.True(t, batchRouterDB.callOrder > processor.callOrder)
		require.True(t, errorDB.callOrder > processor.callOrder)

		t.Log("dbs should be stopped after router")
		require.True(t, gatewayDB.callOrder > router.callOrder)
		require.True(t, routerDB.callOrder > router.callOrder)
		require.True(t, batchRouterDB.callOrder > router.callOrder)
		require.True(t, errorDB.callOrder > router.callOrder)
	})

	t.Run("Shutdown from Normal ", func(t *testing.T) {
		cancel()
		<-wait

		require.Equal(t, "stop", gatewayDB.status)
		require.Equal(t, "stop", routerDB.status)
		require.Equal(t, "stop", batchRouterDB.status)
		require.Equal(t, "stop", errorDB.status)

		require.Equal(t, "stop", processor.status)
		require.Equal(t, "stop", router.status)

		t.Log("dbs should be stopped after processor")
		require.True(t, gatewayDB.callOrder > processor.callOrder)
		require.True(t, routerDB.callOrder > processor.callOrder)
		require.True(t, batchRouterDB.callOrder > processor.callOrder)
		require.True(t, errorDB.callOrder > processor.callOrder)

		t.Log("dbs should be stopped after router")
		require.True(t, gatewayDB.callOrder > router.callOrder)
		require.True(t, routerDB.callOrder > router.callOrder)
		require.True(t, batchRouterDB.callOrder > router.callOrder)
		require.True(t, errorDB.callOrder > router.callOrder)

	})

}

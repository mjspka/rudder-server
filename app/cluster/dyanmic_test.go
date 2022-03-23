package cluster_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/rudderlabs/rudder-server/app/cluster"
)


//var _ cluster.ModeChange = &mockModeChange{}

//type mockModeChange struct {
//	value string
//	acked bool
//	wait  chan bool
//}

//func (m *mockModeChange) Value() string {
//	return m.value
//}
//
//func (m *mockModeChange) Ack() error {
//	m.acked = true
//	close(m.wait)
//	return nil
//}

//func (m *mockModeChange) WaitAck() {
//	<-m.wait
//}

func newMode(newMode, ackKey string) cluster.ModeUpdate {
	return cluster.ModeUpdate{
		ExpectedMode: newMode,
		AckKey:       ackKey,
	}
}

type mockModeProvider struct {
	ch chan cluster.ModeUpdate
}

func (m *mockModeProvider) ServerMode() <-chan cluster.ModeUpdate {
	return m.ch
}

func (m *mockModeProvider) SendMode(newMode cluster.ModeUpdate) {
	m.ch <- newMode
}

func TestDynamicCluster(t *testing.T) {
	provider := &mockModeProvider{ch: make(chan cluster.ModeUpdate)}

	dc := cluster.Dynamic{
		Provider: provider,
	}

	ctx, cancel := context.WithCancel(context.Background())

	wait := make(chan bool)
	go func() {
		dc.Run(ctx)
		close(wait)
	}()

	newMode := newMode("testMode", "testAckKey")
	provider.SendMode(newMode)
	require.Equal(t, "testMode", dc.Mode)
	cancel()
	<-wait
}

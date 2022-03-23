package cluster

import "context"

type ModeChange interface {
	Value() string
	Ack() error
}

type ModeUpdate struct {
	ExpectedMode string
	AckKey       string
}

type modeProvider interface {
	ServerMode() <-chan ModeUpdate
}

type Dynamic struct {
	Mode     string
	Provider modeProvider
}

func (d *Dynamic) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case newMode := <-d.Provider.ServerMode():
			err := d.handleModeChange(newMode.ExpectedMode)
			if err != nil {

			}
			d.handleAck(newMode.AckKey)
		}
	}
}

func (d *Dynamic) handleModeChange(newMode string) error {
	d.Mode = newMode
	return nil
}

func (d *Dynamic) handleAck(key string) error {
	return nil
}

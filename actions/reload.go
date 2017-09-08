package actions

import (
	"../proxy"
)

// Reloader defines the interface for reloading HAProxy
type Reloader interface {
	Execute(recreate bool) error
}

type reload struct{}

// Execute runs the reload.
// If `recreate` is set to `true`, configuration will be recreated before the reload.
func (m *reload) Execute(recreate bool) error {
	if recreate {
		if err := proxy.Instance.CreateConfigFromTemplates(); err != nil {
			logPrintf(err.Error())
			return err
		}
	}
	if err := proxy.Instance.Reload(); err != nil {
		logPrintf(err.Error())
		return err
	}
	return nil
}

// NewReload returns a new instance of the struct
var NewReload = func() Reloader {
	return &reload{}
}

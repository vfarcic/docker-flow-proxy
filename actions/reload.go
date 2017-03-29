package actions

import (
	"../proxy"

)

type Reloader interface {
	Execute(recreate bool) error
}

type Reload struct{}



func (m *Reload) Execute(recreate bool) error {
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

var NewReload = func() Reloader {
	return &Reload{}
}


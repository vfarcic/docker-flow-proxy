package proxy

import (
	"github.com/mitchellh/mapstructure"
	"reflect"
	"strings"
)

type mapParameterProvider struct {
	theMap *map[string]string
}

func (p *mapParameterProvider) Fill(service *Service) {
	mapstructure.Decode(p.theMap, service)
	//above library does not handle bools as strings
	v := reflect.ValueOf(service).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).CanSet() && v.Field(i).Kind() == reflect.Bool {
			fieldName := v.Type().Field(i).Name
			value := ""
			if len(p.GetString(fieldName)) > 0 {
				value = p.GetString(fieldName)
			} else if len(p.GetString(lowerFirst(fieldName))) > 0 {
				value = p.GetString(lowerFirst(fieldName))
			}
			value = strings.ToLower(value)
			if strings.EqualFold(value, "true") {
				v.Field(i).SetBool(true)
			} else if strings.EqualFold(value, "false") {
				v.Field(i).SetBool(false)
			}
		}
	}
}

func (p *mapParameterProvider) GetString(name string) string {
	return (*p.theMap)[name]
}

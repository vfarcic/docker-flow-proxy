package server

type reloadParams struct {
	Recreate     bool `schema:"recreate"`
	FromListener bool `schema:"fromListener"`
}

type removeParams struct {
	AclName     string `schema:"aclName"`
	Distribute  bool   `schema:"distribute"`
	ServiceName string `schema:"serviceName"`
}

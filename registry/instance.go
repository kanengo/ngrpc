package registry

import (
	"fmt"
)

type ServiceInstance struct {
	//ID unique instance ID
	ID string `json:"id"`

	Name string `json:"name"`

	Version string `json:"version"`

	Metadata map[string]string `json:"metadata"`

	Endpoints []string `json:"endpoints"`
}

func (i *ServiceInstance) String() string {
	return fmt.Sprintf("%s.%s", i.Name, i.ID)
}

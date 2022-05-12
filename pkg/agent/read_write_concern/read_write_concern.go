package agent

type ReadWriteConcern struct {
	DefaultReadConcern  DefaultReadConcern  `json:"defaultReadConcern"`
	DefaultWriteConcern DefaultWriteConcern `json:"defaultWriteConcern"`
}

type DefaultReadConcern struct {
	Level string `json:"level,omitempty"`
}

type DefaultWriteConcern struct {
	J        bool   `json:"j,omitempty"`
	W        string `json:"w,omitempty"`
	WTimeout int    `json:"wtimeout,omitempty"`
}

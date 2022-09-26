package tidbcloudapi


type Project struct {
	ID              uint64 `json:"id,string"`
	OrgID           uint64 `json:"org_id,string"`
	Name            string `json:"name"`
	ClusterCount    int64  `json:"cluster_count"`
	UserCount       int64  `json:"user_count"`
	CreateTimestamp int64  `json:"create_timestamp,string"`
}

type ConnectionString struct {
	Standard   string `json:"standard"`
	VpcPeering string `json:"vpc_peering"`
}

type IPAccess struct {
	CIDR        string `json:"cidr"`
	Description string `json:"description"`
}

type ComponentTiDB struct {
	NodeSize       string `json:"node_size"`
	StorageSizeGib int    `json:"storage_size_gib,omitempty"`
	NodeQuantity   int    `json:"node_quantity"`
}

type ComponentTiKV struct {
	NodeSize       string `json:"node_size"`
	StorageSizeGib int    `json:"storage_size_gib"`
	NodeQuantity   int    `json:"node_quantity"`
}

type ComponentTiFlash struct {
	NodeSize       string `json:"node_size"`
	StorageSizeGib int32  `json:"storage_size_gib"`
	NodeQuantity   int32  `json:"node_quantity"`
}

type Components struct {
	TiDB    *ComponentTiDB    `json:"tidb,omitempty"`
	TiKV    *ComponentTiKV    `json:"tikv,omitempty"`
	TiFlash *ComponentTiFlash `json:"tiflash,omitempty"`
}

type ClusterConfig struct {
	RootPassword string     `json:"root_password"`
	Port         int32      `json:"port"`
	Components   Components `json:"components"`
	IPAccessList []IPAccess `json:"ip_access_list"`
}

type ClusterStatus struct {
	TidbVersion   string `json:"tidb_version"`
	ClusterStatus string `json:"cluster_status"`
}

type CreateClusterReq struct {
	Name          string        `json:"name"`
	ClusterType   string        `json:"cluster_type"`
	CloudProvider string        `json:"cloud_provider"`
	Region        string        `json:"region"`
	Config        ClusterConfig `json:"config"`
}

type CreateClusterResp struct {
	ClusterID uint64 `json:"id,string"`
	Message   string `json:"message"`
}

type GetAllProjectsResp struct {
	Items []Project `json:"items"`
	Total int64     `json:"total"`
}

type GetClusterResp struct {
	ID                uint64           `json:"id,string"`
	ProjectID         uint64           `json:"project_id,string"`
	Name              string           `json:"name"`
	Port              int32            `json:"port"`
	TiDBVersion       string           `json:"tidb_version"`
	ClusterType       string           `json:"cluster_type"`
	CloudProvider     string           `json:"cloud_provider"`
	Region            string           `json:"region"`
	Status            ClusterStatus    `json:"status"`
	CreateTimestamp   string           `json:"create_timestamp"`
	Config            ClusterConfig    `json:"config"`
	ConnectionStrings ConnectionString `json:"connection_strings"`
}

type Specification struct {
	ClusterType   string `json:"cluster_type"`
	CloudProvider string `json:"cloud_provider"`
	Region        string `json:"region"`
	Tidb          []struct {
		NodeSize          string `json:"node_size"`
		NodeQuantityRange struct {
			Min  int `json:"min"`
			Step int `json:"step"`
		} `json:"node_quantity_range"`
	} `json:"tidb"`
	Tikv []struct {
		NodeSize          string `json:"node_size"`
		NodeQuantityRange struct {
			Min  int `json:"min"`
			Step int `json:"step"`
		} `json:"node_quantity_range"`
		StorageSizeGibRange struct {
			Min int `json:"min"`
			Max int `json:"max"`
		} `json:"storage_size_gib_range"`
	} `json:"tikv"`
	Tiflash []struct {
		NodeSize          string `json:"node_size"`
		NodeQuantityRange struct {
			Step int `json:"step"`
		} `json:"node_quantity_range"`
		StorageSizeGibRange struct {
			Min int `json:"min"`
			Max int `json:"max"`
		} `json:"storage_size_gib_range"`
	} `json:"tiflash"`
}

type GetSpecificationsResp struct {
	Items []Specification `json:"items"`
}

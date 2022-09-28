package tidbcloudapi

import (
	"fmt"
)

const (
	Host = "https://api.tidbcloud.com"
)

// getSpecifications returns all the available specifications
func GetSpecifications() (*GetSpecificationsResp, error) {
	var (
		url    = fmt.Sprintf("%s/api/v1beta/clusters/provider/regions", Host)
		result GetSpecificationsResp
	)

	_, err := DoGET(url, nil, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetDedicatedSpec(specifications *GetSpecificationsResp) (*Specification, error) {
	for _, i := range specifications.Items {
		if i.ClusterType == "DEDICATED" {
			return &i, nil
		}
	}

	return nil, fmt.Errorf("No specification found")
}

// getAllProjects list all projects in current organization
func GetAllProjects() ([]Project, error) {
	var (
		url    = fmt.Sprintf("%s/api/v1beta/projects", Host)
		result GetAllProjectsResp
	)

	_, err := DoGET(url, nil, &result)
	if err != nil {
		return nil, err
	}

	return result.Items, nil
}

func GetProjectByName(projectName string) (*Project, error) {
	_projects, err := GetAllProjects()
	if err != nil {
		return nil, err
	}

	for _, _project := range _projects {
		if _project.Name == projectName {
			return &_project, nil
		}
	}
	return nil, nil
}

func IsValidProjectID(projectID uint64) (bool, error) {
	_projects, err := GetAllProjects()
	if err != nil {
		return false, err
	}

	for _, _project := range _projects {
		if _project.ID == projectID {
			return true, nil
		}
	}
	return false, nil
}

// createDedicatedCluster create a cluster in the given project
func CreateDedicatedCluster(projectID uint64, spec *Specification) (*CreateClusterResp, error) {
	var (
		url    = fmt.Sprintf("%s/api/v1beta/projects/%d/clusters", Host, projectID)
		result CreateClusterResp
	)

	// We have check the boundary in main function
	tidbSpec := spec.Tidb[0]
	tikvSpec := spec.Tikv[0]

	payload := CreateClusterReq{
		Name:          "tidbcloud-sample-1", // NOTE change to your cluster name
		ClusterType:   spec.ClusterType,
		CloudProvider: spec.CloudProvider,
		Region:        spec.Region,
		Config: ClusterConfig{
			RootPassword: "your secret password", // NOTE change to your cluster password, we generate a random password here
			Port:         4000,
			Components: Components{
				TiDB: &ComponentTiDB{
					NodeSize:     tidbSpec.NodeSize,
					NodeQuantity: tidbSpec.NodeQuantityRange.Min,
				},
				TiKV: &ComponentTiKV{
					NodeSize:       tikvSpec.NodeSize,
					StorageSizeGib: tikvSpec.StorageSizeGibRange.Min,
					NodeQuantity:   tikvSpec.NodeQuantityRange.Min,
				},
			},
			IPAccessList: []IPAccess{
				{
					CIDR:        "0.0.0.0/0",
					Description: "Allow Access from Anywhere.",
				},
			},
		},
	}

	_, err := DoPOST(url, payload, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// getClusterByID return detail status of given cluster
func getClusterByID(projectID, clusterID *uint64) (*Cluster, error) {
	var (
		url    = fmt.Sprintf("%s/api/v1beta/projects/%d/clusters/%d", Host, projectID, clusterID)
		result Cluster
	)

	_, err := DoGET(url, nil, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetClusterByName(projectID uint64, clusterName string) (*Cluster, error) {
	var result GetAllClustersResp
	var _arrProjects []uint64

	if projectID == 0 {
		_projects, err := GetAllProjects()
		if err != nil {
			return nil, err
		}
		for _, _project := range _projects {
			_arrProjects = append(_arrProjects, _project.ID)
		}
	} else {
		_arrProjects = append(_arrProjects, projectID)
	}

	for _, _projectID := range _arrProjects {
		url := fmt.Sprintf("%s/api/v1beta/projects/%d/clusters", Host, _projectID)
		_, err := DoGET(url, nil, &result)
		if err != nil {
			return nil, err
		}

		for _, _item := range result.Items {
			if _item.Name == clusterName {
				return &_item, nil
			}
		}
	}
	return nil, nil
}

// deleteClusterByID delete a cluster
func DeleteClusterByID(projectID, clusterID uint64) error {
	url := fmt.Sprintf("%s/api/v1beta/projects/%d/clusters/%d", Host, projectID, clusterID)
	_, err := doDELETE(url, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func ResumeClusterByID(projectID, clusterID uint64) error {
	url := fmt.Sprintf("%s/api/v1beta/projects/%d/clusters/%d", Host, projectID, clusterID)

	var payload ClusterPauseConfig
	payload.Config.Paused = false

	_, err := doPATCH(url, payload, nil)
	if err != nil {
		return err
	}

	return nil
}

func PauseClusterByID(projectID, clusterID uint64) error {
	url := fmt.Sprintf("%s/api/v1beta/projects/%d/clusters/%d", Host, projectID, clusterID)

	var payload ClusterPauseConfig
	payload.Config.Paused = true

	_, err := doPATCH(url, payload, nil)
	if err != nil {
		return err
	}

	return nil
}

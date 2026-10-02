package state

import (
	"encoding/json"
	"testing"
	"time"
)

func baseState() *State {
	return &State{
		Version: "1",
		Resources: []Resource{
			{Type: ResourceTypeCluster, Name: "cluster-a", Region: "eu-west-2", CreatedBy: "alice"},
			{Type: ResourceTypeCluster, Name: "cluster-b", Region: "us-east-1", CreatedBy: "bob"},
			{Type: ResourceTypeService, Name: "svc-1", Cluster: "cluster-a", Region: "eu-west-2", CreatedBy: "alice"},
			{Type: ResourceTypeService, Name: "svc-2", Cluster: "cluster-b", Region: "us-east-1", CreatedBy: "bob"},
		},
	}
}

// --- AddResource ---

func TestAddResource_NewResource(t *testing.T) {
	st := baseState()
	before := len(st.Resources)

	st.AddResource(Resource{Type: ResourceTypeCluster, Name: "cluster-c", Region: "eu-west-1"})

	if len(st.Resources) != before+1 {
		t.Errorf("expected %d resources, got %d", before+1, len(st.Resources))
	}
}

func TestAddResource_Upsert(t *testing.T) {
	st := baseState()
	before := len(st.Resources)
	updated := time.Now()

	// Replace cluster-a with updated metadata
	st.AddResource(Resource{
		Type:      ResourceTypeCluster,
		Name:      "cluster-a",
		Region:    "eu-west-2",
		CreatedBy: "charlie",
		CreatedAt: updated,
	})

	// Count should not change
	if len(st.Resources) != before {
		t.Errorf("upsert should not add new entry: got %d resources, want %d", len(st.Resources), before)
	}

	// Value should be updated
	for _, r := range st.Resources {
		if r.Type == ResourceTypeCluster && r.Name == "cluster-a" {
			if r.CreatedBy != "charlie" {
				t.Errorf("CreatedBy: got %q, want %q", r.CreatedBy, "charlie")
			}
			return
		}
	}
	t.Error("cluster-a not found after upsert")
}

func TestAddResource_ServiceUpsertByNameClusterAndRegion(t *testing.T) {
	st := baseState()
	before := len(st.Resources)

	// svc-1 in cluster-a already exists — should update
	st.AddResource(Resource{Type: ResourceTypeService, Name: "svc-1", Cluster: "cluster-a", Region: "eu-west-2", CreatedBy: "updated"})
	if len(st.Resources) != before {
		t.Errorf("upsert should not change count: got %d, want %d", len(st.Resources), before)
	}

	// svc-1 in cluster-b does NOT exist — should add
	st.AddResource(Resource{Type: ResourceTypeService, Name: "svc-1", Cluster: "cluster-b", Region: "eu-west-2", CreatedBy: "new"})
	if len(st.Resources) != before+1 {
		t.Errorf("new service in different cluster should be added: got %d, want %d", len(st.Resources), before+1)
	}

	// svc-1 in cluster-a but another region is a distinct resource.
	st.AddResource(Resource{Type: ResourceTypeService, Name: "svc-1", Cluster: "cluster-a", Region: "us-east-1", CreatedBy: "new"})
	if len(st.Resources) != before+2 {
		t.Errorf("new service in different region should be added: got %d, want %d", len(st.Resources), before+2)
	}
}

// --- RemoveResource ---

func TestRemoveResource_Existing(t *testing.T) {
	st := baseState()
	before := len(st.Resources)

	st.RemoveResource(ResourceTypeCluster, "cluster-a", "")

	if len(st.Resources) != before-1 {
		t.Errorf("expected %d resources after remove, got %d", before-1, len(st.Resources))
	}
	for _, r := range st.Resources {
		if r.Type == ResourceTypeCluster && r.Name == "cluster-a" {
			t.Error("cluster-a should have been removed")
		}
	}
}

func TestRemoveResource_NonExistent(t *testing.T) {
	st := baseState()
	before := len(st.Resources)

	// Removing something that doesn't exist should be a no-op
	st.RemoveResource(ResourceTypeCluster, "does-not-exist", "")

	if len(st.Resources) != before {
		t.Errorf("remove of non-existent should not change count: got %d, want %d", len(st.Resources), before)
	}
}

func TestRemoveResource_Service(t *testing.T) {
	st := baseState()
	before := len(st.Resources)

	st.RemoveResource(ResourceTypeService, "svc-1", "cluster-a")

	if len(st.Resources) != before-1 {
		t.Errorf("expected %d resources, got %d", before-1, len(st.Resources))
	}
}

// --- FindClusters ---

func TestFindClusters_ReturnsClustersOnly(t *testing.T) {
	st := baseState()
	clusters := st.FindClusters()

	if len(clusters) != 2 {
		t.Errorf("FindClusters: got %d, want 2", len(clusters))
	}
	for _, r := range clusters {
		if r.Type != ResourceTypeCluster {
			t.Errorf("FindClusters returned non-cluster type: %s", r.Type)
		}
	}
}

func TestFindClusters_Empty(t *testing.T) {
	st := &State{Resources: []Resource{}}
	clusters := st.FindClusters()
	if len(clusters) != 0 {
		t.Errorf("expected empty result, got %d", len(clusters))
	}
}

// --- FindServices ---

func TestFindServices_All(t *testing.T) {
	st := baseState()
	services := st.FindServices("")

	if len(services) != 2 {
		t.Errorf("FindServices (all): got %d, want 2", len(services))
	}
}

func TestFindServices_FilteredByCluster(t *testing.T) {
	st := baseState()
	services := st.FindServices("cluster-a")

	if len(services) != 1 {
		t.Errorf("FindServices (cluster-a): got %d, want 1", len(services))
	}
	if services[0].Name != "svc-1" {
		t.Errorf("expected svc-1, got %q", services[0].Name)
	}
}

func TestFindServices_NoMatch(t *testing.T) {
	st := baseState()
	services := st.FindServices("cluster-does-not-exist")

	if len(services) != 0 {
		t.Errorf("expected 0 services, got %d", len(services))
	}
}

func TestExpressServiceConfigurationPersistsInRemoteStateDocument(t *testing.T) {
	st := &State{Version: "1", Resources: []Resource{{
		Type: ResourceTypeExpressService, Name: "api", Cluster: "prod", ARN: "arn:service", Configuration: "serviceName: api\nimage: app:v1\n",
	}}}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	services := restored.FindExpressServices("prod")
	if len(services) != 1 || services[0].Configuration != "serviceName: api\nimage: app:v1\n" {
		t.Fatalf("restored Express state = %#v", services)
	}
}

// --- RemoveClusterAndServices ---

func TestRemoveClusterAndServices(t *testing.T) {
	st := baseState()
	before := len(st.Resources)

	// Removing cluster-a should remove cluster-a itself and svc-1 (which belongs to cluster-a)
	st.RemoveClusterAndServices("cluster-a")

	// Base state has:
	// - cluster-a (cluster) -> remove
	// - cluster-b (cluster) -> keep
	// - svc-1 (service, cluster-a) -> remove
	// - svc-2 (service, cluster-b) -> keep
	expectedCount := before - 2
	if len(st.Resources) != expectedCount {
		t.Errorf("expected %d resources remaining, got %d", expectedCount, len(st.Resources))
	}

	for _, r := range st.Resources {
		if r.Cluster == "cluster-a" || (r.Type == ResourceTypeCluster && r.Name == "cluster-a") {
			t.Errorf("found resource related to cluster-a that should have been removed: %+v", r)
		}
	}
}

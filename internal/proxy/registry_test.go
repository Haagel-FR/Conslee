package proxy

import (
	"testing"

	"conslee/internal/config"
)

func newTestState(name, host string, containers []string) *ServiceState {
	return &ServiceState{
		Config: config.ServiceConfig{
			Name:       name,
			Host:       host,
			Containers: containers,
		},
	}
}

func TestServiceRegistryAddAndGet(t *testing.T) {
	r := NewRegistry()
	s := newTestState("web", "web.example.com", []string{"web-1"})
	r.Add("web.example.com", s)

	got, ok := r.GetByHost("web.example.com")
	if !ok {
		t.Error("GetByHost not found")
	}
	if got.Config.Name != "web" {
		t.Errorf("GetByHost name = %q, want web", got.Config.Name)
	}

	got, ok = r.GetByName("web")
	if !ok {
		t.Error("GetByName not found")
	}
	if got.Config.Host != "web.example.com" {
		t.Errorf("GetByName host = %q, want web.example.com", got.Config.Host)
	}
}

func TestServiceRegistryDelByName(t *testing.T) {
	r := NewRegistry()
	s := newTestState("web", "web.example.com", []string{"web-1"})
	r.Add("web.example.com", s)

	r.DelByName("web")
	if _, ok := r.GetByName("web"); ok {
		t.Error("GetByName after DelByName should fail")
	}
	if _, ok := r.GetByHost("web.example.com"); ok {
		t.Error("GetByHost after DelByName should fail")
	}
}

func TestServiceRegistryUpdateHost(t *testing.T) {
	r := NewRegistry()
	s := newTestState("web", "web.example.com", []string{"web-1"})
	r.Add("web.example.com", s)

	r.UpdateHost(s, "new.example.com")
	if _, ok := r.GetByHost("web.example.com"); ok {
		t.Error("old host should not be found")
	}
	got, ok := r.GetByHost("new.example.com")
	if !ok {
		t.Error("new host should be found")
	}
	if got.Config.Host != "new.example.com" {
		t.Errorf("Config.Host = %q, want new.example.com", got.Config.Host)
	}
}

func TestServiceRegistryAll(t *testing.T) {
	r := NewRegistry()
	s1 := newTestState("web", "web.example.com", []string{"web-1"})
	s2 := newTestState("api", "api.example.com", []string{"api-1"})
	r.Add("web.example.com", s1)
	r.Add("api.example.com", s2)

	all := r.All()
	if len(all) != 2 {
		t.Errorf("All() len = %d, want 2", len(all))
	}
}

func TestServiceRegistryFindContainerConflict(t *testing.T) {
	r := NewRegistry()
	s1 := newTestState("web", "web.example.com", []string{"web-1", "web-2"})
	r.Add("web.example.com", s1)

	t.Run("conflict found via containers", func(t *testing.T) {
		svcName, containerName := r.FindContainerConflict([]string{"web-1"}, "")
		if svcName != "web" {
			t.Errorf("svcName = %q, want web", svcName)
		}
		if containerName != "web-1" {
			t.Errorf("containerName = %q, want web-1", containerName)
		}
	})

	t.Run("no conflict", func(t *testing.T) {
		svcName, containerName := r.FindContainerConflict([]string{"other"}, "")
		if svcName != "" || containerName != "" {
			t.Errorf("expected no conflict, got %q %q", svcName, containerName)
		}
	})

	t.Run("excludes self", func(t *testing.T) {
		svcName, containerName := r.FindContainerConflict([]string{"web-1"}, "web")
		if svcName != "" || containerName != "" {
			t.Errorf("expected no conflict when excluding self, got %q %q", svcName, containerName)
		}
	})

	t.Run("conflict via container_name", func(t *testing.T) {
		s2 := newTestState("db", "db.example.com", nil)
		s2.Config.ContainerName = "db-main"
		r.Add("db.example.com", s2)
		svcName, containerName := r.FindContainerConflict([]string{"db-main"}, "other")
		if svcName != "db" {
			t.Errorf("svcName = %q, want db", svcName)
		}
		if containerName != "db-main" {
			t.Errorf("containerName = %q, want db-main", containerName)
		}
	})

	t.Run("empty containers no conflict", func(t *testing.T) {
		svcName, containerName := r.FindContainerConflict([]string{}, "")
		if svcName != "" || containerName != "" {
			t.Errorf("expected no conflict for empty, got %q %q", svcName, containerName)
		}
	})
}

func TestServiceRegistryAddEmptyHost(t *testing.T) {
	r := NewRegistry()
	s := newTestState("web", "", []string{"web-1"})
	r.Add("", s)

	if _, ok := r.GetByHost(""); ok {
		t.Error("empty host should not be in byHost")
	}
	if _, ok := r.GetByName("web"); !ok {
		t.Error("GetByName should work with empty host")
	}
}

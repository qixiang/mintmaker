package base

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	renovateBaseConfig      map[string]interface{}
	renovateBaseConfigMutex sync.RWMutex
)

type BaseComponent struct {
	Name        string
	Namespace   string
	Application string
	Platform    string
	Host        string
	GitURL      string
	// Path of repository, without hostname
	// Temporary field to make the implementation easy, it's part of GitURL, so they're duplicated
	Repository string
	Branch     string
	Timestamp  int64
}

func (c *BaseComponent) GetName() string {
	return c.Name
}

func (c *BaseComponent) GetNamespace() string {
	return c.Namespace
}

func (c *BaseComponent) GetApplication() string {
	return c.Application
}

func (c *BaseComponent) GetPlatform() string {
	return c.Platform
}

func (c *BaseComponent) GetHost() string {
	return c.Host
}

func (c *BaseComponent) GetGitURL() string {
	return c.GitURL
}

func (c *BaseComponent) GetRepository() string {
	return c.Repository
}

func (c *BaseComponent) GetTimestamp() int64 {
	return c.Timestamp
}

func (c *BaseComponent) GetRenovateBaseConfig(client client.Client, ctx context.Context) (map[string]interface{}, error) {

	if renovateBaseConfig != nil {
		return renovateBaseConfig, nil
	}

	baseConfig := corev1.ConfigMap{}
	configmapKey := types.NamespacedName{Namespace: "mintmaker", Name: "renovate-config"}
	if err := client.Get(ctx, configmapKey, &baseConfig); err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(baseConfig.Data["renovate.json"]), &config); err != nil {
		return nil, fmt.Errorf("error unmarshaling Renovate config: %v", err)
	}

	renovateBaseConfigMutex.Lock()
	renovateBaseConfig = config
	renovateBaseConfigMutex.Unlock()
	return config, nil
}

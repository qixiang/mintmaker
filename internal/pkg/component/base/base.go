package base

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

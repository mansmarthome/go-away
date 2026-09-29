package settings

type WAF struct {
	Enabled        bool   `yaml:"enabled"`
	Engine         string `yaml:"engine"`
	DirectivesFile string `yaml:"directives-file"`
}

package gitnotify

import (
	"io/ioutil"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

// AppConfig is
type AppConfig struct {
	ServerProto         string   `yaml:"serverProto"`       // can be http:// or https://
	ServerHost          string   `yaml:"serverHost"`        // domain.com with port . Used at redirection for OAuth
	LocalHost           string   `yaml:"localHost"`         // host:port combination used for starting the server
	DataDir             string   `yaml:"dataDir"`           // relative path from server to write the data
	SettingsFile        string   `yaml:"settingsFile"`      // name of file to be looked up/saved to for data
	FromName            string   `yaml:"fromName"`          // name of from email user
	FromEmail           string   `yaml:"fromEmail"`         // email address of from email address
	GithubAPIEndPoint   string   `yaml:"githubAPIEndPoint"` // server endpoint with protocol for https://api.github.com
	GithubURLEndPoint   string   `yaml:"githubURLEndPoint"` // website end point https://github.com
	GitlabAPIEndPoint   string   `yaml:"gitlabAPIEndPoint"` // server endpoint with protocol for https://gitlab.com/api/v3/
	GitlabURLEndPoint   string   `yaml:"gitlabURLEndPoint"` // website end point https://gitlab.com
	SMTPHost            string   `yaml:"smtpHost"`
	SMTPPort            int      `yaml:"smtpPort"`
	SMTPSesConfSet      string   `yaml:"sesConfigurationSet"` // ses configuration set used as a custom header while sending email
	GoogleAnalytics     string   `yaml:"googleAnalytics"`
	SMTPUser            string   // environment variable
	SMTPPass            string   // environment variable
	CacheMode           bool     `yaml:"cacheMode"` // when cacheMode is false, views are loaded on every request
	WebhookIntegrations []string `yaml:"webhookIntegrations"`
	StatHatKey          string   `yaml:"stathatKey"`
	StatHatEnvironment  string   `yaml:"stathatEnvironment"` // Environment string is used to track Stats in StatHatKey
	// SentryURL           string   `yaml:"sentryDSN"`

	TemplateDir         string `yaml:"templateDir"`         // tmpl/
	TemplatePartialsDir string `yaml:"templatePartialsDir"` // tmpl/partials/
	// "changes_mail" and "changes_mail_text" are the files used to render
	// "home" for home page, "repos" for the repositories page
	// "text" for rendering simple text
	// "user" for user preferences
	// use "partial name" to render a file

	Providers map[string]string // List of ProviderNames that are configured as per auth

	SourceCodeLink string
}

func (c *AppConfig) serverHostWithoutPort() string {
	return strings.Split(c.ServerHost, ":")[0]
}

func (c *AppConfig) websiteURL() string {
	return c.ServerProto + "://" + c.ServerHost
}

func (c *AppConfig) isEmailSetup() bool {
	return c.SMTPHost != ""
}

func (c *AppConfig) getStatHatPrefix() string {
	if c.StatHatEnvironment != "" {
		return c.StatHatEnvironment + "."
	}
	return "default."
}

var config = new(AppConfig)

// LoadConfig loads the config from the file
func LoadConfig(appConfigFile string) {
	if _, err := os.Stat(appConfigFile); os.IsNotExist(err) {
		panic(err)
	}

	data, err := ioutil.ReadFile(appConfigFile)
	if os.IsNotExist(err) {
		panic(err)
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		panic(err)
	}

	if config.isEmailSetup() {
		config.SMTPUser = os.Getenv("SMTP_USER")
		config.SMTPPass = os.Getenv("SMTP_PASS")

		// dont send email, but start the server but not the email daemon
		if config.SMTPUser == "" {
			panic("Missing Configuration: SMTP username is not set!")
		}
		if config.SMTPPass == "" {
			panic("Missing Configuration: SMTP password is not set!")
		}
	}

	InitSession()
	InitView()
	initTZ()
	preInitAuth()

	// variables used by views

	if config.Providers[GithubProvider] != "" {
		githubRepoEndPoint = config.GithubURLEndPoint + "%s/"                      // repo/abc
		githubTreeURLEndPoint = config.GithubURLEndPoint + "%s/tree/%s"            // repo/abc , develop
		githubCommitURLEndPoint = config.GithubURLEndPoint + "%s/commits/%s"       // repo/abc , develop
		githubCompareURLEndPoint = config.GithubURLEndPoint + "%s/compare/%s...%s" // repo/abc, base, target commit ref
	}

	if config.Providers[GitlabProvider] != "" {
		gitlabRepoEndPoint = config.GitlabURLEndPoint + "%s/"                      // repo/abc
		gitlabTreeURLEndPoint = config.GitlabURLEndPoint + "%s/tree/%s"            // repo/abc , develop
		gitlabCommitURLEndPoint = config.GitlabURLEndPoint + "%s/commits/%s"       // repo/abc , develop
		gitlabCompareURLEndPoint = config.GitlabURLEndPoint + "%s/compare/%s...%s" // repo/abc, base, target commit ref
	}

	config.SourceCodeLink = "https://github.com/sairam/gitnotify"
}

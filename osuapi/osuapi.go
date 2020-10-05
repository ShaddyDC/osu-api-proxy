package osuapi

// OsuAPI connects to the osu api v2
type OsuAPI struct {
	config   Config
	Handlers map[string]APIFunc
}

// Config contains the config stuff of the api
type Config struct {
	ClientID     int    `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURI  string `mapstructure:"redirect_uri"`
}

// NewOsuAPI creates a new api instance
func NewOsuAPI(config Config) OsuAPI {
	osuAPI := &OsuAPI{
		config: config}

	osuAPI.setupHandlers()

	return *osuAPI
}

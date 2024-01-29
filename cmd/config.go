package cmd

type Config struct {
	App []App `yaml:"service"`
	Log Log   `yaml:"log"`
}

type Nacos struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type App struct {
	// Names    application name
	Names []string `yaml:"names"`
	Env   string   `yaml:"env"`
	Nacos Nacos    `yaml:"nacos"`
	Keep  []Meta   `yaml:"keep"`
}

type Meta struct {
	Key string `yaml:"key"`
	Val string `yaml:"value"`
}

type Log struct {
	Level string `yaml:"level"`
}

/**

service:
  - names:
	  - App1
	  - App2
    env: dev
	nacos:
	  addr: 127.0.0.0
	  username: admin
	  password: 123456
  - names:
	  - App3
	  - App4
	env: prod
	nacos:
	  addr:
	  username:
	  password:

**
*/

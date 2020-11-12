package sq

type TableConfig struct {
}

func (t *TableConfig) Field(field Field, fieldType string, constraints ...func() string) {
}

func (t *TableConfig) ForeignKey(foreign, primary Field, constraints ...func() string) {
}

func (t *TableConfig) Unique(fields ...Field) {
}

type Constraints struct{}

func (c Constraints) PrimaryKey() string { return "PRIMARY KEY" }

func (c Constraints) NotNull() string { return "NOT NULL" }

func (c Constraints) Serial() string { return "SERIAL" }

func (c Constraints) Unique() string { return "UNIQUE" }

func (c Constraints) Default(defaultValue string) func() string {
	return func() string { return defaultValue }
}

func (c Constraints) OnUpdateCascade() string { return "ON UPDATE CASCADE" }

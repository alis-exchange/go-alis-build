package validation

type StringField struct {
	path  string
	value func() string
}

func (s *StringField) Populated() *Rule {
	return &Rule{
		Description: s.path + " must be populated",
		satisfied:   func() bool { return s.value() != "" },
	}
}

func (s *StringField) Empty() *Rule {
	return &Rule{
		Description: s.path + " must be empty",
		satisfied:   func() bool { return s.value() == "" },
	}
}

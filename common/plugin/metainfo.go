package plugin

type Meta struct {
	Name      string
	HelpWords string
	AdminOnly bool
	Order     int
}

func (p *Meta) GetMeta() *Meta {
	return p
}

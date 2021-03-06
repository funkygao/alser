package plugins

import (
	"fmt"
	"github.com/funkygao/dpipe/engine"
	conf "github.com/funkygao/jsconf"
)

type cardinalityField struct {
	key       string
	typ       string
	intervals []string
}

type cardinalityConverter struct {
	logPrefix string
	project   string
	fields    []cardinalityField
}

func (this *cardinalityConverter) load(section *conf.Conf) {
	this.logPrefix = section.String("log_prefix", "")
	this.project = section.String("project", "")
	this.fields = make([]cardinalityField, 0, 5)
	for i := 0; i < len(section.List("fields", nil)); i++ {
		keyPrefix := fmt.Sprintf("fields[%d].", i)
		field := cardinalityField{}
		field.key = section.String(keyPrefix+"key", "")
		field.typ = section.String(keyPrefix+"type", "string")
		field.intervals = section.StringList(keyPrefix+"intervals", nil)
		this.fields = append(this.fields, field)
	}
}

type CardinalityFilter struct {
	ident      string
	converters []cardinalityConverter
}

func (this *CardinalityFilter) Init(config *conf.Conf) {
	this.ident = config.String("ident", "")
	if this.ident == "" {
		panic("empty ident")
	}
	for i := 0; i < len(config.List("converts", nil)); i++ {
		section, err := config.Section(fmt.Sprintf("%s[%d]", "converts", i))
		if err != nil {
			panic(err)
		}

		c := cardinalityConverter{}
		c.load(section)
		this.converters = append(this.converters, c)
	}
}

func (this *CardinalityFilter) Run(r engine.FilterRunner,
	h engine.PluginHelper) error {
	var (
		pack   *engine.PipelinePack
		ok     = true
		inChan = r.InChan()
	)

LOOP:
	for ok {
		select {
		case pack, ok = <-inChan:
			if !ok {
				break LOOP
			}

			this.handlePack(r, h, pack)
			pack.Recycle()
		}
	}

	return nil
}

// for each inbound pack, this filter will generate several new pack
// the original pack will be recycled immediately
func (this *CardinalityFilter) handlePack(r engine.FilterRunner,
	h engine.PluginHelper, pack *engine.PipelinePack) {
	globals := engine.Globals()
	for _, c := range this.converters {
		if !pack.Logfile.MatchPrefix(c.logPrefix) || pack.Project != c.project {
			continue
		}

		for _, f := range c.fields {
			val, err := pack.Message.FieldValue(f.key, f.typ)
			if err != nil {
				if globals.Verbose {
					h.Project(c.project).Println(err)
				}

				return
			}

			for _, interval := range f.intervals {
				// generate new pack
				p := h.PipelinePack(pack.MsgLoopCount)
				if p == nil {
					globals.Println("can't get pack in filter")
					continue
				}

				p.Ident = this.ident
				p.Project = c.project
				p.CardinalityKey = fmt.Sprintf("%s.%s.%s", pack.Project, f.key, interval)
				p.CardinalityData = val
				p.CardinalityInterval = interval

				r.Inject(p)
			}
		}
	}
}

func init() {
	engine.RegisterPlugin("CardinalityFilter", func() engine.Plugin {
		return new(CardinalityFilter)
	})
}

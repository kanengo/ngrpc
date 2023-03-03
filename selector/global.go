package selector

var globalSelector = &wrapSelector{}

var _ Builder = (*wrapSelector)(nil)

type wrapSelector struct {
	Builder
}

func GlobalSelectorBuilder() Builder {
	if globalSelector.Builder != nil {
		return globalSelector.Builder
	}

	return nil
}

func SetGlobalSelector(builder Builder) {
	globalSelector.Builder = builder
}

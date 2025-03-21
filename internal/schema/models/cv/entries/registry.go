package entries

type TypeName string

type Descriptor struct {
	Name   TypeName
	Fields []string
}

type Registry struct {
	descriptors []Descriptor
}

func NewRegistry(in ...Descriptor) *Registry {
	return &Registry{descriptors: in}
}

func (r *Registry) Descriptors() []Descriptor {
	return r.descriptors
}

func (r *Registry) Names() []TypeName {
	names := make([]TypeName, len(r.descriptors))
	for i, d := range r.descriptors {
		names[i] = d.Name
	}
	return append(names, "TextEntry")
}

func (r *Registry) Characteristic() map[TypeName]map[string]struct{} {
	allAttrs := make(map[string]int)
	for _, d := range r.descriptors {
		for _, f := range d.Fields {
			allAttrs[f]++
		}
	}

	common := make(map[string]bool)
	for attr, count := range allAttrs {
		if count > 1 {
			common[attr] = true
		}
	}

	result := make(map[TypeName]map[string]struct{})
	for _, d := range r.descriptors {
		chars := make(map[string]struct{})
		for _, f := range d.Fields {
			if !common[f] {
				chars[f] = struct{}{}
			}
		}
		result[d.Name] = chars
	}
	return result
}

func (r *Registry) Discriminate(keys []string) (TypeName, bool) {
	chars := r.Characteristic()
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}

	for _, d := range r.descriptors {
		for f := range chars[d.Name] {
			if keySet[f] {
				return d.Name, true
			}
		}
	}
	return "", false
}

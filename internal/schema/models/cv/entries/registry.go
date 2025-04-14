package entries

// TypeName is an entry type's name, as it appears in user-facing messages.
type TypeName string

// Descriptor is one entry type's registration: its name and its full field-name
// set, including inherited names.
type Descriptor struct {
	Name   TypeName
	Fields []string
}

// Registry holds the entry types section discrimination runs against. Iteration
// 3 populates it with the real types (spec §7.1).
type Registry struct {
	descriptors []Descriptor
}

// NewRegistry builds a registry from descriptors in priority order (spec §3.57).
func NewRegistry(in ...Descriptor) *Registry {
	return &Registry{descriptors: in}
}

// Descriptors returns the registered types in priority order.
func (r *Registry) Descriptors() []Descriptor {
	return r.descriptors
}

// Names returns the registered type names followed by TextEntry, which exists
// as a type but not as a model (spec §3.57).
func (r *Registry) Names() []TypeName {
	names := make([]TypeName, len(r.descriptors))
	for i, d := range r.descriptors {
		names[i] = d.Name
	}
	return append(names, TextEntryName)
}

// Characteristic computes each type's characteristic fields: those declared by
// exactly one type (spec §3.55).
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

// Discriminate returns the first type in priority order whose characteristic
// fields intersect the given keys (spec §3.57).
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

// Package report is the deadset report document as Go values, generated from
// contract/report.schema.json and contract/finding.schema.json, and the rules
// every product applies to it: [Decode] and [Encode], which refuse what the
// schemas refuse; [Order], the canonical order of every array; the counts of
// [Totals]; [Cap]; and the [Build] an analyzer assembles its report with.
//
// Admission of a decoded report against a schema range and a conformance
// result is the reader's decision, not this package's.
package report

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// Configuration is one entry of the build matrix, in one of the two shapes the
// schema admits: a platform, or a project naming one compiler configuration
// file. Exactly one of Platform and Project is set.
type Configuration struct {
	Platform *Platform
	ID       string
	Project  string
}

// Platform is the platform shape of a build configuration.
type Platform struct {
	OS   string
	Arch string
	Tags []string
}

// ConfigurationNotBuilt is one configuration an analysis derived, could not
// build and dropped, with the first line of the error that dropped it.
type ConfigurationNotBuilt struct {
	Error string
	Configuration
}

// configurationMembers is an entry as a document writes it, in the schema's
// member order for both shapes.
//
//nolint:govet // fieldalignment: the field order is the member order the encoding writes
type configurationMembers struct {
	ID      string    `json:"id"`
	OS      *string   `json:"os,omitzero"`
	Arch    *string   `json:"arch,omitzero"`
	Tags    *[]string `json:"tags,omitzero"`
	Project string    `json:"project,omitzero"`
	Error   *string   `json:"error,omitzero"`
}

func (c *Configuration) members() configurationMembers {
	written := configurationMembers{ID: c.ID, Project: c.Project}
	if c.Platform != nil {
		written.OS, written.Arch, written.Tags = &c.Platform.OS, &c.Platform.Arch, &c.Platform.Tags
	}
	return written
}

func (m *configurationMembers) configuration() Configuration {
	held := Configuration{ID: m.ID, Project: m.Project}
	if m.OS != nil || m.Arch != nil || m.Tags != nil {
		held.Platform = &Platform{}
		if m.OS != nil {
			held.Platform.OS = *m.OS
		}
		if m.Arch != nil {
			held.Platform.Arch = *m.Arch
		}
		if m.Tags != nil {
			held.Platform.Tags = *m.Tags
		}
	}
	return held
}

// MarshalJSON writes the entry in its own shape.
func (c Configuration) MarshalJSON() ([]byte, error) { return marshal(c.members()) }

// UnmarshalJSON reads an entry of either shape; [Decode] has held it to the
// schema first.
func (c *Configuration) UnmarshalJSON(data []byte) error {
	var read configurationMembers
	if err := json.Unmarshal(data, &read); err != nil {
		return err
	}
	*c = read.configuration()
	return nil
}

// MarshalJSON writes the entry in its own shape, followed by its error.
func (c ConfigurationNotBuilt) MarshalJSON() ([]byte, error) {
	written := c.members()
	written.Error = &c.Error
	return marshal(written)
}

// UnmarshalJSON reads an entry of either shape with its error.
func (c *ConfigurationNotBuilt) UnmarshalJSON(data []byte) error {
	var read configurationMembers
	if err := json.Unmarshal(data, &read); err != nil {
		return err
	}
	*c = ConfigurationNotBuilt{Configuration: read.configuration()}
	if read.Error != nil {
		c.Error = *read.Error
	}
	return nil
}

// KindOf is the kind the target gives language: its member of LanguageKinds
// where the kind is mixed, and the kind otherwise.
func (t *Target) KindOf(language vocab.Language) TargetKind {
	if t.Kind != TargetKindMixed {
		return t.Kind
	}
	return t.LanguageKinds[language]
}

// Line is the withheld line contract/grammar/text-line.md states, empty where
// nothing is withheld. No minimum withholds a certain finding.
func (w *Withheld) Line() string {
	var named []string
	lowest := ""
	for _, count := range []struct {
		confidence vocab.Class
		n          int
	}{{vocab.Probable, w.Probable}, {vocab.Possible, w.Possible}} {
		if count.n > 0 {
			named = append(named, strconv.Itoa(count.n)+" "+string(count.confidence))
			lowest = string(count.confidence)
		}
	}
	if named == nil {
		return ""
	}
	return "withheld by analysis.min_confidence: " + strings.Join(named, ", ") +
		", shown with analysis.min_confidence set to " + lowest
}

// Members is the members each listed component holds, by its identifier.
func (r *Report) Members() map[string][]Positioned {
	members := make(map[string][]Positioned, len(r.Components))
	for i := range r.Components {
		members[r.Components[i].ID] = r.Components[i].Members
	}
	return members
}

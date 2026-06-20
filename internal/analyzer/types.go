package analyzer

import "time"

const Version = "0.1.1"

type Report struct {
	Version     string           `json:"version"`
	GeneratedAt time.Time        `json:"generated_at"`
	Inputs      []InputFile      `json:"inputs"`
	Summary     Summary          `json:"summary"`
	Missions    []MissionSummary `json:"missions"`
	Ops         []OpSummary      `json:"ops"`
	Timeline    []TimelineEvent  `json:"timeline"`
	Findings    []Finding        `json:"findings"`
	Redactions  map[string]int   `json:"redactions"`
	Surface     SpecKittySurface `json:"spec_kitty_surface"`
	Notes       []string         `json:"notes"`
}

type InputFile struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Bytes int64  `json:"bytes"`
}

type Summary struct {
	InputFiles     int `json:"input_files"`
	Turns          int `json:"turns"`
	TimelineEvents int `json:"timeline_events"`
	Missions       int `json:"missions"`
	Ops            int `json:"ops"`
	OpenOps        int `json:"open_ops"`
	SlashCommands  int `json:"slash_commands"`
	UniqueCommands int `json:"unique_slash_commands"`
	CLIInvocations int `json:"cli_invocations"`
	UniqueCLIVerbs int `json:"unique_cli_verbs"`
	Skills         int `json:"skills"`
	UniqueSkills   int `json:"unique_skills"`
	AgentProfiles  int `json:"agent_profiles"`
	FailureEvents  int `json:"failure_events"`
	FailureModes   int `json:"failure_modes"`
	MissionEvents  int `json:"mission_events"`
	OpEvents       int `json:"op_events"`
	OutsideEvents  int `json:"outside_events"`
}

type Scope struct {
	Type         string `json:"type"`
	MissionSlug  string `json:"mission_slug,omitempty"`
	InvocationID string `json:"invocation_id,omitempty"`
	WorkPackage  string `json:"work_package,omitempty"`
	Action       string `json:"action,omitempty"`
}

type TimelineEvent struct {
	Seq            int                  `json:"seq"`
	Turn           int                  `json:"turn"`
	Timestamp      *time.Time           `json:"timestamp,omitempty"`
	SourcePath     string               `json:"source_path"`
	Line           int                  `json:"line,omitempty"`
	Kind           string               `json:"kind"`
	Title          string               `json:"title"`
	Scope          Scope                `json:"scope"`
	TextPreview    string               `json:"text_preview,omitempty"`
	ToolName       string               `json:"tool_name,omitempty"`
	SlashCommands  []SlashCommand       `json:"slash_commands"`
	CLIInvocations []CLIInvocation      `json:"cli_invocations"`
	Skills         []SkillUse           `json:"skills"`
	AgentProfiles  []AgentProfileUse    `json:"agent_profiles"`
	Failures       []FailureFingerprint `json:"failures"`
	RawJSONKeys    []string             `json:"raw_json_keys,omitempty"`
}

type SlashCommand struct {
	Name   string `json:"name"`
	Action string `json:"action,omitempty"`
	Raw    string `json:"raw"`
}

type CLIInvocation struct {
	Raw             string   `json:"raw"`
	Args            []string `json:"args"`
	Verb            string   `json:"verb,omitempty"`
	Subcommand      string   `json:"subcommand,omitempty"`
	Mission         string   `json:"mission,omitempty"`
	WorkPackage     string   `json:"work_package,omitempty"`
	Agent           string   `json:"agent,omitempty"`
	Profile         string   `json:"profile,omitempty"`
	SaaSSyncEnabled bool     `json:"saas_sync_enabled"`
}

type SkillUse struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	Raw  string `json:"raw"`
}

type AgentProfileUse struct {
	Profile string `json:"profile"`
	Agent   string `json:"agent,omitempty"`
	Role    string `json:"role,omitempty"`
	Raw     string `json:"raw"`
}

type FailureFingerprint struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Severity      string `json:"severity"`
	Reason        string `json:"reason"`
	Recovery      string `json:"recovery,omitempty"`
	Deterministic bool   `json:"deterministic"`
}

type Finding struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Severity      string            `json:"severity"`
	Count         int               `json:"count"`
	Scopes        []Scope           `json:"scopes"`
	FirstSeq      int               `json:"first_seq"`
	LastSeq       int               `json:"last_seq"`
	Evidence      []FindingEvidence `json:"evidence"`
	Recovery      string            `json:"recovery,omitempty"`
	Deterministic bool              `json:"deterministic"`
}

type FindingEvidence struct {
	Seq        int    `json:"seq"`
	SourcePath string `json:"source_path"`
	Line       int    `json:"line,omitempty"`
	Text       string `json:"text"`
}

type MissionSummary struct {
	Slug           string             `json:"slug"`
	MissionType    string             `json:"mission_type,omitempty"`
	TargetBranch   string             `json:"target_branch,omitempty"`
	Files          []string           `json:"files"`
	WorkPackages   []WorkPackageState `json:"work_packages"`
	SlashCommands  []string           `json:"slash_commands"`
	CLIInvocations []string           `json:"cli_invocations"`
	Skills         []string           `json:"skills"`
	FailureModes   []string           `json:"failure_modes"`
	EventCount     int                `json:"event_count"`
	FailureCount   int                `json:"failure_count"`
}

type WorkPackageState struct {
	ID           string   `json:"id"`
	Lane         string   `json:"lane,omitempty"`
	ReviewStatus string   `json:"review_status,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	SourcePath   string   `json:"source_path,omitempty"`
}

type OpSummary struct {
	InvocationID string     `json:"invocation_id"`
	Status       string     `json:"status"`
	ProfileID    string     `json:"profile_id,omitempty"`
	Action       string     `json:"action,omitempty"`
	Outcome      string     `json:"outcome,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Files        []string   `json:"files"`
	FailureModes []string   `json:"failure_modes"`
	EventCount   int        `json:"event_count"`
}

type SpecKittySurface struct {
	Version          string   `json:"version,omitempty"`
	TopLevelCommands []string `json:"top_level_commands"`
	SlashCommands    []string `json:"slash_commands"`
	SkillFamilies    []string `json:"skill_families"`
	DecisionKinds    []string `json:"decision_kinds"`
	MissionTypes     []string `json:"mission_types"`
	WPLanes          []string `json:"wp_lanes"`
}

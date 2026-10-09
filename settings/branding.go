package settings

// Branding holds how the interface looks. Gezgin's brand is fixed: File Browser's instance name,
// colour, branding folder (custom styles and images) and external links option are gone, and a
// database that still has them loads without them.
type Branding struct {
	Theme                 string `json:"theme"`
	DisableUsedPercentage bool   `json:"disableUsedPercentage"`
}

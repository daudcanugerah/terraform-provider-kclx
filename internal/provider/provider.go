// internal/provider/provider.go
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &kclProvider{}

type kclProvider struct {
	// Add provider configuration fields here
	KclPath string
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &kclProvider{version: version}
	}
}

func (p *kclProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "kcl"
	resp.Version = p.version
}

func (p *kclProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"kcl_path": schema.StringAttribute{
				Optional:    true,
				Description: "Path to the KCL executable",
			},
		},
	}
}

func (p *kclProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config struct {
		KclPath types.String `tfsdk:"kcl_path"`
	}

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set the provider configuration
	if !config.KclPath.IsNull() {
		p.KclPath = config.KclPath.ValueString()
	}

	// Make the provider configuration available to resources
	resp.ResourceData = p
}

func (p *kclProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewKclExecResource,
	}
}

func (p *kclProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

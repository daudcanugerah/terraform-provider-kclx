// internal/provider/kcl_exec_resource.go
package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces
var (
	_ resource.Resource              = &KclExecResource{}
	_ resource.ResourceWithConfigure = &KclExecResource{}
)

func NewKclExecResource() resource.Resource {
	return &KclExecResource{}
}

type KclExecResource struct {
	provider *kclProvider
}

type KclExecResourceModel struct {
	ID          types.String `tfsdk:"id"`
	SourceDir   types.String `tfsdk:"source_dir"`
	Output      types.String `tfsdk:"output"`
	Args        types.List   `tfsdk:"args"`
	Triggers    types.Map    `tfsdk:"triggers"`
	Timeout     types.Int64  `tfsdk:"timeout"`
	Environment types.Map    `tfsdk:"environment"`
}

func (r *KclExecResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exec"
}

func (r *KclExecResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Executes KCL (Kusion Configuration Language) scripts in a specified directory",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the execution",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"source_dir": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Path to directory containing KCL scripts",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"output": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Combined standard output and error from KCL execution",
			},
			"args": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Additional arguments to pass to KCL command",
				PlanModifiers:       []planmodifier.List{},
			},
			"triggers": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Map of values that should trigger re-execution when changed",
				PlanModifiers:       []planmodifier.Map{},
			},
			"timeout": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Execution timeout in seconds (default: 300)",
				PlanModifiers:       []planmodifier.Int64{},
			},
			"environment": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Environment variables to set during execution",
				PlanModifiers:       []planmodifier.Map{},
			},
		},
	}
}

func (r *KclExecResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*kclProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *kclProvider, got: %T", req.ProviderData),
		)
		return
	}

	r.provider = provider
}

func (r *KclExecResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan KclExecResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate and resolve source directory
	sourceDir := plan.SourceDir.ValueString()
	absPath, err := filepath.Abs(sourceDir)
	if err != nil {
		resp.Diagnostics.AddError("Path Resolution Error", "Invalid source directory path: "+err.Error())
		return
	}

	// Check directory existence
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		resp.Diagnostics.AddError("Directory Not Found", "Source directory does not exist: "+absPath)
		return
	}

	// Determine KCL command path
	kclCommand := "kcl"
	if r.provider != nil && r.provider.KclPath != "" {
		kclCommand = r.provider.KclPath
	}

	// Prepare arguments
	args := []string{}
	if !plan.Args.IsNull() {
		diags := plan.Args.ElementsAs(ctx, &args, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Prepare environment variables
	envVars := os.Environ()
	if !plan.Environment.IsNull() {
		envMap := make(map[string]string)
		diags := plan.Environment.ElementsAs(ctx, &envMap, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for k, v := range envMap {
			envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Create execution context with timeout
	timeout := 300 * time.Second
	if !plan.Timeout.IsNull() {
		timeout = time.Duration(plan.Timeout.ValueInt64()) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(ctx, kclCommand, args...)
	cmd.Dir = absPath
	cmd.Env = envVars

	tflog.Info(ctx, "Executing KCL command", map[string]interface{}{
		"command":   kclCommand,
		"arguments": args,
		"directory": absPath,
		"timeout":   timeout,
	})

	output, err := cmd.CombinedOutput()
	if err != nil {
		resp.Diagnostics.AddError(
			"KCL Execution Failed",
			fmt.Sprintf("Command: %s %s\nError: %v\nOutput: %s",
				kclCommand, strings.Join(args, " "), err, string(output)),
		)
		return
	}

	// Generate unique ID based on inputs
	idInput := fmt.Sprintf("%s|%s|%v|%v", absPath, kclCommand, args, envVars)
	hash := sha256.Sum256([]byte(idInput))
	plan.ID = types.StringValue(hex.EncodeToString(hash[:16]))
	plan.Output = types.StringValue(strings.TrimSpace(string(output)))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *KclExecResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Output is ephemeral - nothing to read after creation
}

func (r *KclExecResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"KCL execution resource does not support updates - changes require replacement",
	)
}

func (r *KclExecResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No persistent state to clean up
}

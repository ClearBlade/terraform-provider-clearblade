package clearblade

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/clearblade/go-iot"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &deviceResource{}
	_ resource.ResourceWithConfigure   = &deviceResource{}
	_ resource.ResourceWithImportState = &deviceResource{}
)

func NewDeviceResource() resource.Resource {
	return &deviceResource{}
}

// deviceResource is the resource implementation.
type deviceResource struct {
	client *iot.Service
}

type deviceResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	NumID              types.String `tfsdk:"num_id"`
	Credentials        types.List   `tfsdk:"credentials"`
	LastHeartbeatTime  types.String `tfsdk:"last_heartbeat_time"`
	LastEventTime      types.String `tfsdk:"last_event_time"`
	LastStateTime      types.String `tfsdk:"last_state_time"`
	LastConfigAckTime  types.String `tfsdk:"last_config_ack_time"`
	LastConfigSendTime types.String `tfsdk:"last_config_send_time"`
	Blocked            types.Bool   `tfsdk:"blocked"`
	LastErrorTime      types.String `tfsdk:"last_error_time"`
	LastErrorStatus    types.Object `tfsdk:"last_error_status"`
	Config             types.Object `tfsdk:"config"`
	State              types.Object `tfsdk:"state"`
	LogLevel           types.String `tfsdk:"log_level"`
	Metadata           types.Map    `tfsdk:"metadata"`
	GatewayConfig      types.Object `tfsdk:"gateway_config"`
	Registry           types.String `tfsdk:"registry"`
}

type DeviceCredentialsModel struct {
	PublicKeyCertificate DevicePublicKeyCertificateModel `tfsdk:"public_key_certificate"`
}

type DevicePublicKeyCertificateModel struct {
	ExpirationTime types.String   `tfsdk:"expiration_time"`
	PublicKey      PublicKeyModel `tfsdk:"public_key"`
}

var DevicePublicKeyCertificateModelTypes = map[string]attr.Type{
	"expiration_time": types.StringType,
	"public_key":      types.ObjectType{AttrTypes: PublicKeyModelTypes},
}

type PublicKeyModel struct {
	Format types.String `tfsdk:"format"`
	Key    types.String `tfsdk:"key"`
}

var PublicKeyModelTypes = map[string]attr.Type{
	"format": types.StringType,
	"key":    types.StringType,
}

type LastErrorStatusModel struct {
	Code types.Int64 `tfsdk:"code"`
	// Details []detailsModel `tfsdk:"details"`
	Message types.String `tfsdk:"message"`
}

var LastErrorStatusModelTypes = map[string]attr.Type{
	"code":    types.Int64Type,
	"message": types.StringType,
}

type MetadataModel struct {
}

var MetadataModelTypes = map[string]attr.Type{}

// type detailsModel struct{}

type ConfigModel struct {
	Version         types.Int64  `tfsdk:"version"`
	CloudUpdateTime types.String `tfsdk:"cloud_update_time"`
	DeviceAckTime   types.String `tfsdk:"device_ack_time"`
	BinaryData      types.String `tfsdk:"binary_data"`
}

var ConfigModelTypes = map[string]attr.Type{
	"version":           types.Int64Type,
	"cloud_update_time": types.StringType,
	"device_ack_time":   types.StringType,
	"binary_data":       types.StringType,
}

type StateModel struct {
	UpdateTime types.String `tfsdk:"update_time"`
	BinaryData types.String `tfsdk:"binary_data"`
}

var StateModelTypes = map[string]attr.Type{
	"update_time": types.StringType,
	"binary_data": types.StringType,
}

type GatewayConfigModel struct {
	GatewayType             types.String `tfsdk:"gateway_type"`
	GatewayAuthMethod       types.String `tfsdk:"gateway_auth_method"`
	LastAccessedGatewayID   types.String `tfsdk:"last_accessed_gateway_id"`
	LastAccessedGatewayTime types.String `tfsdk:"last_accessed_gateway_time"`
}

var GatewayConfigModelTypes = map[string]attr.Type{
	"gateway_type":               types.StringType,
	"gateway_auth_method":        types.StringType,
	"last_accessed_gateway_id":   types.StringType,
	"last_accessed_gateway_time": types.StringType,
}

// Schema defines the schema for the resource.
func (r *deviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The user-defined device identifier. The device ID must be unique within a device registry.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "The resource path name. For example, projects/p1/locations/us-central1/registries/registry0/devices/dev0 or projects/p1/locations/us-central1/registries/registry0/devices/{numId}.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"num_id": schema.StringAttribute{
				Description: "A server-defined unique numeric ID for the device. This is a more compact way to identify devices, and it is globally unique.",
				Computed:    true,
			},
			"credentials": schema.ListNestedAttribute{
				Optional: true,
				// Computed:    true,
				Description: "The credentials used to authenticate this device.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"expiration_time": schema.StringAttribute{
							Optional:    true,
							Description: "The time at which this credential becomes invalid.",
						},
						"public_key": schema.SingleNestedAttribute{
							Optional:            true,
							MarkdownDescription: "A public key used to verify the signature of JSON Web Tokens (JWTs).",
							Attributes: map[string]schema.Attribute{
								"format": schema.StringAttribute{
									Optional: true,
									Validators: []validator.String{
										stringvalidator.OneOf(
											"RSA_PEM",
											"RSA_X509_PEM",
											"ES256_PEM",
											"ES256_X509_PEM",
										),
									},
									Description: `The format of the key. Possible values: ["RSA_PEM", "RSA_X509_PEM", "ES256_PEM", "ES256_X509_PEM"]`,
								},
								"key": schema.StringAttribute{
									Optional:    true,
									Description: "The key data.",
								},
							},
						},
					},
				},
			},
			"last_heartbeat_time": schema.StringAttribute{
				MarkdownDescription: "The last time an MQTT PINGREQ was received.",
				Computed:            true,
			},
			"last_event_time": schema.StringAttribute{
				Description: "The last time a telemetry event was received.",
				Computed:    true,
			},
			"last_state_time": schema.StringAttribute{
				MarkdownDescription: "The last time a state event was received.",
				Computed:            true,
			},
			"last_config_ack_time": schema.StringAttribute{
				Description: "The last time a cloud-to-device config version acknowledgment was received from the device.",
				Computed:    true,
			},
			"last_config_send_time": schema.StringAttribute{
				Description: "The last time a cloud-to-device config version was sent to the device.",
				Computed:    true,
			},
			"blocked": schema.BoolAttribute{
				Description: "If a device is blocked, connections or requests from this device will fail.",
				Optional:    true,
				Computed:    true,
			},
			"last_error_time": schema.StringAttribute{
				Description: "The time the most recent error occurred, such as a failure to publish to Cloud Pub/Sub.",
				Computed:    true,
			},
			"last_error_status": schema.SingleNestedAttribute{
				Description: "The error message of the most recent error, such as a failure to publish to Cloud Pub/Sub.",
				Optional:    true,
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"code": schema.Int64Attribute{
						Optional:    true,
						Computed:    true,
						Description: `The status code, which should be an enum value of google.rpc.Code.`,
					},
					"message": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: `A developer-facing error message, which should be in English.`,
					},
				},
			},
			"config": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The most recent device configuration, which is eventually sent from Cloud IoT Core to the device.",
				Attributes: map[string]schema.Attribute{
					"version": schema.Int64Attribute{
						Optional:    true,
						Computed:    true,
						Description: `The version of this update.`,
					},
					"cloud_update_time": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: `The time at which this configuration version was updated in Cloud IoT Core.`,
					},
					"device_ack_time": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: `The time at which Cloud IoT Core received the acknowledgment from the device, indicating that the device has received this configuration version.`,
					},
					"binary_data": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: `The device configuration data.`,
					},
				},
			},
			"state": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The state most recently received from the device.",
				Attributes: map[string]schema.Attribute{
					"update_time": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: `The time at which this state version was updated in Cloud IoT Core.`,
					},
					"binary_data": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: `The device state data.`,
					},
				},
			},
			"log_level": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"NONE",
						"ERROR",
						"INFO",
						"DEBUG",
						"",
					),
				},
				Description: `The logging verbosity for device activity. Possible values: ["NONE", "ERROR", "INFO", "DEBUG"]`,
			},
			"metadata": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
			},
			"gateway_config": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: `Gateway-related configuration and state.`,
				Attributes: map[string]schema.Attribute{
					"gateway_type": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Validators: []validator.String{
							stringvalidator.OneOf(
								"GATEWAY",
								"NON_GATEWAY",
								"",
							),
						},
						Description: `Indicates whether the device is a gateway. Default value: "NON_GATEWAY" Possible values: ["GATEWAY", "NON_GATEWAY"]`,
					},
					"gateway_auth_method": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Validators: []validator.String{
							stringvalidator.OneOf(
								"ASSOCIATION_ONLY",
								"DEVICE_AUTH_TOKEN_ONLY",
								"ASSOCIATION_AND_DEVICE_AUTH_TOKEN",
							),
						},
						Description: `Indicates whether the device is a gateway. Possible values: ["ASSOCIATION_ONLY", "DEVICE_AUTH_TOKEN_ONLY", "ASSOCIATION_AND_DEVICE_AUTH_TOKEN"]`,
					},
					"last_accessed_gateway_id": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: `The ID of the gateway the device accessed most recently.`,
					},
					"last_accessed_gateway_time": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: `The most recent time at which the device accessed the gateway specified in last_accessed_gateway.`,
					},
				},
			},
			"registry": schema.StringAttribute{
				Description: "The name of the device registry where this device should be created.",
				Required:    true,
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *deviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating iot device resource")

	// Retrieve values from plan
	var plan deviceResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		tflog.Debug(ctx, "error in the diagnostics")
		return
	}

	// Generate API request body from plan
	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"), plan.Registry.ValueString())
	device, err := r.client.Projects.Locations.Registries.Devices.Create(parent, createDeviceApiRequestFromPlan(ctx, &plan)).Do()

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating a device",
			"Could not create a new device, unexpected error: "+err.Error(),
		)
		return
	}

	mapCbDeviceToPlanSchema(ctx, &plan, device)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *deviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading the device resource")

	// Get current state
	var state deviceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed device detail from ClearBlade IoT Core
	str := state.ID.ValueString()
	re := regexp.MustCompile(`/`)
	slice := re.Split(str, -1)
	if len(slice) > 1 {
		state.Registry = types.StringValue(slice[0])
		state.ID = types.StringValue(slice[1])
	}

	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"), state.Registry.ValueString(), state.ID.ValueString())
	device, err := r.client.Projects.Locations.Registries.Devices.Get(parent).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading ClearBlade IoT Core device detail",
			"Could not read info about ClearBlade IoT Core device detail, unexpected error: "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite items with refreshed state
	state.Name = types.StringValue(device.Name)
	state.LastConfigAckTime = types.StringValue(device.LastConfigAckTime)
	state.LastConfigSendTime = types.StringValue(device.LastConfigSendTime)
	state.LastErrorTime = types.StringValue(device.LastErrorTime)
	state.LastEventTime = types.StringValue(device.LastEventTime)
	state.LastHeartbeatTime = types.StringValue(device.LastHeartbeatTime)
	state.LastStateTime = types.StringValue(device.LastStateTime)
	state.NumID = types.StringValue(strconv.FormatUint(device.NumId, 10))
	state.Blocked = types.BoolValue(device.Blocked)
	state.LogLevel = types.StringValue(device.LogLevel)

	if state.State.IsNull() {
		attributes := map[string]attr.Value{
			"update_time": types.StringNull(),
			"binary_data": types.StringNull(),
		}
		state.State = types.ObjectValueMust(StateModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"update_time": types.StringValue(device.State.UpdateTime),
			"binary_data": types.StringValue(device.State.BinaryData),
		}
		state.State = types.ObjectValueMust(StateModelTypes, attributes)
	}

	if state.LastErrorStatus.IsNull() {
		attributes := map[string]attr.Value{
			"code":    types.Int64Null(),
			"message": types.StringNull(),
		}
		state.LastErrorStatus = types.ObjectValueMust(LastErrorStatusModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"code":    types.Int64Value(device.LastErrorStatus.Code),
			"message": types.StringValue(device.LastErrorStatus.Message),
		}
		state.LastErrorStatus = types.ObjectValueMust(LastErrorStatusModelTypes, attributes)
	}

	if state.GatewayConfig.IsNull() {
		attributes := map[string]attr.Value{
			"gateway_type":               types.StringNull(),
			"gateway_auth_method":        types.StringNull(),
			"last_accessed_gateway_id":   types.StringNull(),
			"last_accessed_gateway_time": types.StringNull(),
		}
		state.GatewayConfig = types.ObjectValueMust(GatewayConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"gateway_type":               types.StringValue(device.GatewayConfig.GatewayType),
			"gateway_auth_method":        types.StringValue(device.GatewayConfig.GatewayAuthMethod),
			"last_accessed_gateway_id":   types.StringValue(device.GatewayConfig.LastAccessedGatewayId),
			"last_accessed_gateway_time": types.StringValue(device.GatewayConfig.LastAccessedGatewayTime),
		}
		state.GatewayConfig = types.ObjectValueMust(GatewayConfigModelTypes, attributes)
	}

	if state.Config.IsNull() {
		attributes := map[string]attr.Value{
			"version":           types.Int64Null(),
			"cloud_update_time": types.StringNull(),
			"device_ack_time":   types.StringNull(),
			"binary_data":       types.StringNull(),
		}
		state.Config = types.ObjectValueMust(ConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"version":           types.Int64Value(device.Config.Version),
			"cloud_update_time": types.StringValue(device.Config.CloudUpdateTime),
			"device_ack_time":   types.StringValue(device.Config.DeviceAckTime),
			"binary_data":       types.StringValue(device.Config.BinaryData),
		}
		state.Config = types.ObjectValueMust(ConfigModelTypes, attributes)
	}

	attributes := map[string]attr.Value{}
	for k, v := range device.Metadata {
		s, _ := strconv.Unquote(v)
		ctx = tflog.SetField(ctx, "state attributes ll", s)
		tflog.Debug(ctx, "state attributes read")
		attributes[k] = types.StringValue(s)
	}
	state.Metadata, _ = types.MapValueFrom(ctx, state.Metadata.ElementType(ctx), attributes)

	if state.Credentials.IsNull() {
		tflog.Debug(ctx, "value detected NULL - READ")
		state.Credentials = types.ListNull(state.Credentials.ElementType(ctx))
	} else {
		tflog.Debug(ctx, "value detected KNOWN - READ")
		state.Credentials, _ = types.ListValueFrom(ctx, state.Credentials.ElementType(ctx), createPlanDeviceCredsFromCb(device.Credentials))
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *deviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Debug(ctx, "Updating iot device resource")

	// Retrieve values from plan
	var plan deviceResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"), plan.Registry.ValueString(), plan.ID.ValueString())
	device, err := r.client.Projects.Locations.Registries.Devices.Patch(parent, createDeviceApiRequestFromPlan(ctx, &plan)).
		UpdateMask(`blocked,credentials,gatewayConfig.gatewayAuthMethod,logLevel,metadata`).Do()
	// Could not create a new device, unexpected error: googleapi: Error 400: The field mask 'updateMask' must contain mutable fields. The following fields are mutable: ["blocked","credentials","gatewayConfig.gatewayAuthMethod","logLevel","metadata"]

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating a device",
			"Could not create a new device, unexpected error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "device updated")
	tflog.Info(ctx, fmt.Sprintf("Device after update: %+v", device))

	// Update device resource - Map response body to schema and populate Computed attribute values
	mapCbDeviceToPlanSchema(ctx, &plan, device)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *deviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Deleting a device resource")

	// Retrieve values from state
	var state deviceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing device resource on ClearBlade IoT Core
	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"), state.Registry.ValueString(), state.ID.ValueString())
	_, err := r.client.Projects.Locations.Registries.Devices.Delete(parent).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Clearblade IoT Core device",
			"Could not delete device, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *deviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Metadata returns the data source type name.
func (r *deviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iot_device"
}

// Configure adds the provider configured client to the data source.
func (r *deviceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*iot.Service)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *iot.Service, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func createPlanDeviceCredsFromCb(deviceCreds []*iot.DeviceCredential) []DeviceCredentialsModel {
	var credentials = make([]DeviceCredentialsModel, 0)

	for _, credential := range deviceCreds {
		m := DeviceCredentialsModel{
			PublicKeyCertificate: DevicePublicKeyCertificateModel{
				ExpirationTime: types.StringValue(credential.ExpirationTime),
				PublicKey: PublicKeyModel{
					Format: types.StringValue(credential.PublicKey.Format),
					Key:    types.StringValue(credential.PublicKey.Key),
				},
			},
		}
		credentials = append(credentials, m)
	}

	return credentials
}

func createDeviceApiRequestFromPlan(ctx context.Context, plan *deviceResourceModel) *iot.Device {
	credentials := []*iot.DeviceCredential{}

	if len(plan.Credentials.Elements()) > 0 {
		var credentialsModel []DeviceCredentialsModel

		plan.Credentials.ElementsAs(ctx, &credentialsModel, false)

		for _, v := range credentialsModel {
			credentials = append(credentials, &iot.DeviceCredential{
				ExpirationTime: v.PublicKeyCertificate.ExpirationTime.ValueString(),
				PublicKey: &iot.PublicKeyCredential{
					Format: v.PublicKeyCertificate.PublicKey.Format.ValueString(),
					Key:    v.PublicKeyCertificate.PublicKey.Key.ValueString(),
				},
			})
		}
	}

	var gatewayConfigModel GatewayConfigModel
	plan.GatewayConfig.As(ctx, &gatewayConfigModel, basetypes.ObjectAsOptions{})

	metadaAttribute := plan.Metadata.Elements()
	convMetadata := make(map[string]string)
	for k, v := range metadaAttribute {
		convMetadata[k] = v.String()
	}

	return &iot.Device{
		Id:          plan.ID.ValueString(),
		Credentials: credentials,
		Blocked:     plan.Blocked.ValueBool(),
		LogLevel:    plan.LogLevel.ValueString(),
		Metadata:    convMetadata,
		GatewayConfig: &iot.GatewayConfig{
			GatewayAuthMethod:       gatewayConfigModel.GatewayAuthMethod.ValueString(),
			GatewayType:             gatewayConfigModel.GatewayType.ValueString(),
			LastAccessedGatewayId:   gatewayConfigModel.LastAccessedGatewayID.ValueString(),
			LastAccessedGatewayTime: gatewayConfigModel.LastAccessedGatewayTime.ValueString(),
		},
	}
}

func mapCbDeviceToPlanSchema(ctx context.Context, plan *deviceResourceModel, device *iot.Device) {
	plan.Name = types.StringValue(device.Name)

	plan.LastConfigAckTime = types.StringValue(device.LastConfigAckTime)
	plan.LastConfigSendTime = types.StringValue(device.LastConfigSendTime)
	plan.LastErrorTime = types.StringValue(device.LastErrorTime)
	plan.LastEventTime = types.StringValue(device.LastEventTime)
	plan.LastHeartbeatTime = types.StringValue(device.LastHeartbeatTime)
	plan.LastStateTime = types.StringValue(device.LastStateTime)
	plan.NumID = types.StringValue(strconv.FormatUint(device.NumId, 10))
	plan.Blocked = types.BoolValue(device.Blocked)
	plan.LogLevel = types.StringValue(device.LogLevel)

	if plan.State.IsNull() {
		attributes := map[string]attr.Value{
			"update_time": types.StringNull(),
			"binary_data": types.StringNull(),
		}
		plan.State = types.ObjectValueMust(StateModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"update_time": types.StringValue(device.State.UpdateTime),
			"binary_data": types.StringValue(device.State.BinaryData),
		}
		plan.State = types.ObjectValueMust(StateModelTypes, attributes)
	}

	if plan.LastErrorStatus.IsNull() {
		attributes := map[string]attr.Value{
			"code":    types.Int64Null(),
			"message": types.StringNull(),
		}
		plan.LastErrorStatus = types.ObjectValueMust(LastErrorStatusModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"code":    types.Int64Value(device.LastErrorStatus.Code),
			"message": types.StringValue(device.LastErrorStatus.Message),
		}
		plan.LastErrorStatus = types.ObjectValueMust(LastErrorStatusModelTypes, attributes)
	}

	if plan.GatewayConfig.IsNull() {
		attributes := map[string]attr.Value{
			"gateway_type":               types.StringNull(),
			"gateway_auth_method":        types.StringNull(),
			"last_accessed_gateway_id":   types.StringNull(),
			"last_accessed_gateway_time": types.StringNull(),
		}
		plan.GatewayConfig = types.ObjectValueMust(GatewayConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"gateway_type":               types.StringValue(device.GatewayConfig.GatewayType),
			"gateway_auth_method":        types.StringValue(device.GatewayConfig.GatewayAuthMethod),
			"last_accessed_gateway_id":   types.StringValue(device.GatewayConfig.LastAccessedGatewayId),
			"last_accessed_gateway_time": types.StringValue(device.GatewayConfig.LastAccessedGatewayTime),
		}
		plan.GatewayConfig = types.ObjectValueMust(GatewayConfigModelTypes, attributes)
	}

	if plan.Config.IsNull() {
		attributes := map[string]attr.Value{
			"version":           types.Int64Null(),
			"cloud_update_time": types.StringNull(),
			"device_ack_time":   types.StringNull(),
			"binary_data":       types.StringNull(),
		}
		plan.Config = types.ObjectValueMust(ConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"version":           types.Int64Value(device.Config.Version),
			"cloud_update_time": types.StringValue(device.Config.CloudUpdateTime),
			"device_ack_time":   types.StringValue(device.Config.DeviceAckTime),
			"binary_data":       types.StringValue(device.Config.BinaryData),
		}
		plan.Config = types.ObjectValueMust(ConfigModelTypes, attributes)
	}

	if plan.Credentials.IsNull() {
		tflog.Debug(ctx, "value detected NULL - CREATE")
		plan.Credentials = types.ListNull(plan.Credentials.ElementType(ctx))
	} else {
		plan.Credentials, _ = types.ListValueFrom(ctx, plan.Credentials.ElementType(ctx), createPlanDeviceCredsFromCb(device.Credentials))
	}

	if plan.Metadata.IsNull() {
		plan.Metadata = types.MapNull(plan.Metadata.ElementType(ctx))
	} else {
		attributes := map[string]attr.Value{}
		for k, v := range device.Metadata {
			s, _ := strconv.Unquote(v)
			ctx = tflog.SetField(ctx, "state attributes", s)
			attributes[k] = types.StringValue(s)
		}
		plan.Metadata, _ = types.MapValueFrom(ctx, plan.Metadata.ElementType(ctx), attributes)
	}
}

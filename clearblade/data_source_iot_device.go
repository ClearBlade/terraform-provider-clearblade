package clearblade

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/clearblade/go-iot"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &devicesDataSource{}
	_ datasource.DataSourceWithConfigure = &devicesDataSource{}
)

// devicesDataSourceModel maps the data source schema data.
type devicesDataSourceModel struct {
	Devices  []devicesModel `tfsdk:"devices"`
	Registry types.String   `tfsdk:"registry"`
}

// devicesModel maps device schema data.
type devicesModel struct {
	ID          types.String             `tfsdk:"id"`
	Name        types.String             `tfsdk:"name"`
	NumID       types.String             `tfsdk:"num_id"`
	Credentials []DeviceCredentialsModel `tfsdk:"credentials"`
	// Credentials        []DevicePublicKeyCertificateModel `tfsdk:"credentials"`
	LastHeartbeatTime  types.String         `tfsdk:"last_heartbeat_time"`
	LastEventTime      types.String         `tfsdk:"last_event_time"`
	LastStateTime      types.String         `tfsdk:"last_state_time"`
	LastConfigAckTime  types.String         `tfsdk:"last_config_ack_time"`
	LastConfigSendTime types.String         `tfsdk:"last_config_send_time"`
	Blocked            types.Bool           `tfsdk:"blocked"`
	LastErrorTime      types.String         `tfsdk:"last_error_time"`
	LastErrorStatus    LastErrorStatusModel `tfsdk:"last_error_status"`
	Config             ConfigModel          `tfsdk:"config"`
	State              StateModel           `tfsdk:"state"`
	LogLevel           types.String         `tfsdk:"log_level"`
	// Metadata           types.Map                         `tfsdk:"metadata"`
	GatewayConfig GatewayConfigModel `tfsdk:"gateway_config"`
}

func NewDevicesDataSource() datasource.DataSource {
	return &devicesDataSource{}
}

type devicesDataSource struct {
	client *iot.Service
}

func (d *devicesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List devices in a device registry.",
		Attributes: map[string]schema.Attribute{
			"devices": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "The user-defined device identifier. The device ID must be unique within a device registry.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "The resource path name. For example, projects/p1/locations/us-central1/registries/registry0/devices/dev0 or projects/p1/locations/us-central1/registries/registry0/devices/{numId}.",
						},
						"num_id": schema.StringAttribute{
							Computed:    true,
							Description: "A server-defined unique numeric ID for the device. This is a more compact way to identify devices, and it is globally unique.",
						},
						"credentials": schema.ListNestedAttribute{
							Computed:    true,
							Description: "The credentials used to authenticate this device.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"expiration_time": schema.StringAttribute{
										Computed:    true,
										Description: "The time at which this credential becomes invalid.",
									},
									"public_key": schema.SingleNestedAttribute{
										Optional:            true,
										MarkdownDescription: "A public key used to verify the signature of JSON Web Tokens (JWTs).",
										Attributes: map[string]schema.Attribute{
											"format": schema.StringAttribute{
												Computed: true,
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
												Computed:    true,
												Description: "The key data.",
											},
										},
									},
								},
							},
						},
						"last_heartbeat_time": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The last time an MQTT PINGREQ was received.",
						},
						"last_event_time": schema.StringAttribute{
							Computed:    true,
							Description: "The last time a telemetry event was received.",
						},
						"last_state_time": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The last time a state event was received.",
						},
						"last_config_ack_time": schema.StringAttribute{
							Computed:    true,
							Description: "The last time a cloud-to-device config version acknowledgment was received from the device.",
						},
						"last_config_send_time": schema.StringAttribute{
							Computed:    true,
							Description: "The last time a cloud-to-device config version was sent to the device.",
						},
						"blocked": schema.BoolAttribute{
							Computed:    true,
							Description: "If a device is blocked, connections or requests from this device will fail.",
						},
						"last_error_time": schema.StringAttribute{
							Computed:    true,
							Description: "The time the most recent error occurred, such as a failure to publish to Cloud Pub/Sub.",
						},
						"last_error_status": schema.SingleNestedAttribute{
							Computed:    true,
							Description: "The error message of the most recent error, such as a failure to publish to Cloud Pub/Sub.",
							Attributes: map[string]schema.Attribute{
								"code": schema.Int64Attribute{
									Computed:    true,
									Description: `The status code, which should be an enum value of google.rpc.Code.`,
								},
								"message": schema.StringAttribute{
									Computed:    true,
									Description: `A developer-facing error message, which should be in English.`,
								},
							},
						},
						"config": schema.SingleNestedAttribute{
							Computed:    true,
							Description: "The most recent device configuration, which is eventually sent from Cloud IoT Core to the device.",
							Attributes: map[string]schema.Attribute{
								"version": schema.Int64Attribute{
									Computed:    true,
									Description: `The version of this update.`,
								},
								"cloud_update_time": schema.StringAttribute{
									Computed:    true,
									Description: `The time at which this configuration version was updated in Cloud IoT Core.`,
								},
								"device_ack_time": schema.StringAttribute{
									Computed:    true,
									Description: `The time at which Cloud IoT Core received the acknowledgment from the device, indicating that the device has received this configuration version.`,
								},
								"binary_data": schema.StringAttribute{
									Computed:    true,
									Description: `The device configuration data.`,
								},
							},
						},
						"state": schema.SingleNestedAttribute{
							Computed:    true,
							Description: "The state most recently received from the device.",
							Attributes: map[string]schema.Attribute{
								"update_time": schema.StringAttribute{
									Computed:    true,
									Description: `The time at which this state version was updated in Cloud IoT Core.`,
								},
								"binary_data": schema.StringAttribute{
									Computed:    true,
									Description: `The device state data.`,
								},
							},
						},
						"log_level": schema.StringAttribute{
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
						// "metadata": schema.MapAttribute{
						// 	Computed:    true,
						// 	ElementType: types.StringType,
						// },
						"gateway_config": schema.SingleNestedAttribute{
							Computed:    true,
							Description: `Gateway-related configuration and state.`,
							Attributes: map[string]schema.Attribute{
								"gateway_type": schema.StringAttribute{
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
									Computed:    true,
									Description: `The ID of the gateway the device accessed most recently.`,
								},
								"last_accessed_gateway_time": schema.StringAttribute{
									Computed:    true,
									Description: `The most recent time at which the device accessed the gateway specified in last_accessed_gateway.`,
								},
							},
						},
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

func (d *devicesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state devicesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	tflog.Info(ctx, "requesting device listing from Clearblade IoT Core")
	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"), state.Registry.ValueString())
	devices, err := d.client.Projects.Locations.Registries.Devices.List(parent).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read Clearblade IoT Core devices. Make sure your credentials are correct and you have access "+
				"to the project, or that you have the correct permissions.",
			err.Error(),
		)
		return
	}

	ctx = tflog.SetField(ctx, "device length", strconv.Itoa(len(devices.Devices)))

	// Map response body to model
	for _, device := range devices.Devices {
		deviceState := devicesModel{
			ID:    types.StringValue(device.Id),
			Name:  types.StringValue(device.Name),
			NumID: types.StringValue(strconv.FormatUint(device.NumId, 10)),
		}

		for _, credential := range device.Credentials {
			deviceState.Credentials = append(deviceState.Credentials, DeviceCredentialsModel{
				ExpirationTime: types.StringValue(credential.ExpirationTime),
				PublicKey: PublicKeyModel{
					Format: types.StringValue(credential.PublicKey.Format),
					Key:    types.StringValue(credential.PublicKey.Key),
				},
			})
		}

		deviceState.LastHeartbeatTime = types.StringValue(device.LastHeartbeatTime)
		deviceState.LastEventTime = types.StringValue(device.LastEventTime)
		deviceState.LastStateTime = types.StringValue(device.LastStateTime)
		deviceState.LastConfigAckTime = types.StringValue(device.LastConfigAckTime)
		deviceState.LastConfigSendTime = types.StringValue(device.LastConfigSendTime)
		deviceState.Blocked = types.BoolValue(device.Blocked)
		deviceState.LastErrorTime = types.StringValue(device.LastErrorTime)

		deviceState.LastErrorStatus = LastErrorStatusModel{
			Code:    types.Int64Value(device.LastErrorStatus.Code),
			Message: types.StringValue(device.LastErrorStatus.Message),
		}

		deviceState.Config = ConfigModel{
			Version:         types.Int64Value(device.Config.Version),
			CloudUpdateTime: types.StringValue(device.Config.CloudUpdateTime),
			DeviceAckTime:   types.StringValue(device.Config.DeviceAckTime),
			BinaryData:      types.StringValue(device.Config.BinaryData),
		}

		deviceState.State = StateModel{
			UpdateTime: types.StringValue(device.State.UpdateTime),
			BinaryData: types.StringValue(device.State.BinaryData),
		}

		deviceState.LogLevel = types.StringValue(device.LogLevel)

		// attributes := map[string]attr.Value{}
		// for k, v := range device.Metadata {
		// 	s, _ := strconv.Unquote(v)
		// 	ctx = tflog.SetField(ctx, "state attributes ll", s)
		// 	tflog.Debug(ctx, "state attributes read")
		// 	attributes[k] = types.StringValue(s)
		// }
		// deviceState.Metadata, _ = types.MapValueFrom(ctx, state.Metadata.ElementType(ctx), attributes)

		deviceState.GatewayConfig = GatewayConfigModel{
			GatewayType:             types.StringValue(device.GatewayConfig.GatewayType),
			GatewayAuthMethod:       types.StringValue(device.GatewayConfig.GatewayAuthMethod),
			LastAccessedGatewayID:   types.StringValue(device.GatewayConfig.LastAccessedGatewayId),
			LastAccessedGatewayTime: types.StringValue(device.GatewayConfig.LastAccessedGatewayTime),
		}

		state.Devices = append(state.Devices, deviceState)
	}

	// Set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *devicesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_devices"
}

// Configure adds the provider configured client to the data source.
func (d *devicesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.client = req.ProviderData.(*iot.Service)
}

package clearblade

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/clearblade/go-iot"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &deviceRegistryResource{}
	_ resource.ResourceWithConfigure   = &deviceRegistryResource{}
	_ resource.ResourceWithImportState = &deviceRegistryResource{}
)

type deviceRegistryResourceModel struct {
	Timeouts                 timeouts.Value                  `tfsdk:"timeouts"`
	ID                       types.String                    `tfsdk:"id"`
	Name                     types.String                    `tfsdk:"name"`
	EventNotificationConfigs []EventNotificationConfigsModel `tfsdk:"event_notification_configs"`
	StateNotificationConfig  types.Object                    `tfsdk:"state_notification_config"`
	MqttConfig               types.Object                    `tfsdk:"mqtt_config"`
	HttpConfig               types.Object                    `tfsdk:"http_config"`
	LogLevel                 types.String                    `tfsdk:"log_level"`
	Credentials              types.List                      `tfsdk:"credentials"`
	// Credentials              types.Set                       `tfsdk:"credentials"`
	// Credentials              []CredentialsModel              `tfsdk:"credentials"`
	// Region                   types.String                    `tfsdk:"region"`
	// Project                  types.String                    `tfsdk:"project"`
	// LastUpdated              types.String                    `tfsdk:"last_updated"`
}

type EventNotificationConfigsModel struct {
	PubsubTopicName  types.String `tfsdk:"pubsub_topic_name"`
	SubfolderMatches types.String `tfsdk:"subfolder_matches"`
	SubFolderMatches types.String `tfsdk:"sub_folder_matches"`
}

type StateNotificationConfigModel struct {
	PubsubTopicName types.String `tfsdk:"pubsub_topic_name"`
}

var StateNotificationConfigModelTypes = map[string]attr.Type{
	"pubsub_topic_name": types.StringType,
}

type MqttConfigModel struct {
	MqttEnabledState types.String `tfsdk:"mqtt_enabled_state"`
}

var MqttConfigModelTypes = map[string]attr.Type{
	"mqtt_enabled_state": types.StringType,
}

type HttpConfigModel struct {
	HttpEnabledState types.String `tfsdk:"http_enabled_state"`
}

var HttpConfigModelTypes = map[string]attr.Type{
	"http_enabled_state": types.StringType,
}

type CredentialsModel struct {
	PublicKeyCertificate PublicKeyCertificateModel `tfsdk:"public_key_certificate"`
}

type PublicKeyCertificateModel struct {
	Format      types.String                `tfsdk:"format"`
	Certificate types.String                `tfsdk:"certificate"`
	X509Details X509CertificateDetailsModel `tfsdk:"x509_details"`
}

type X509CertificateDetailsModel struct {
	Issuer             types.String `tfsdk:"issuer"`
	Subject            types.String `tfsdk:"subject"`
	StartTime          types.String `tfsdk:"start_time"`
	ExpiryTime         types.String `tfsdk:"expiry_time"`
	SignatureAlgorithm types.String `tfsdk:"signature_algorithm"`
	PublicKeyType      types.String `tfsdk:"public_key_type"`
}

func NewDeviceRegistryResource() resource.Resource {
	return &deviceRegistryResource{}
}

// deviceRegistryResource is the resource implementation.
type deviceRegistryResource struct {
	client *iot.Service
}

// Schema defines the schema for the resource.
func (r *deviceRegistryResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
			}),
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier of this device registry. For example, myRegistry.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The resource path name. For example, projects/example-project/locations/us-central1/registries/my-registry.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"event_notification_configs": schema.ListNestedAttribute{
				Optional: true,
				// Computed: true,
				MarkdownDescription: "The configuration for notification of telemetry events received from the device. " +
					"All telemetry events that were successfully published by the device and acknowledged by Clearblade IoT Core are guaranteed to be delivered to Cloud Pub/Sub.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"pubsub_topic_name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "A Cloud Pub/Sub topic name. For example, projects/myProject/topics/deviceEvents.",
						},
						"sub_folder_matches": schema.StringAttribute{
							Optional: true,
							Computed: true,
							MarkdownDescription: "If the subfolder name matches this string exactly, this configuration will be used. The string must not include the leading '/' character. If empty, all strings are matched. " +
								"This field is used only for telemetry events; subfolders are not supported for state changes.",
						},
						"subfolder_matches": schema.StringAttribute{
							Optional: true,
							Computed: true,
							MarkdownDescription: "If the subfolder name matches this string exactly, this configuration will be used. The string must not include the leading '/' character. If empty, all strings are matched. " +
								"This field is used only for telemetry events; subfolders are not supported for state changes.",
						},
					},
				},
			},
			"state_notification_config": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The configuration for notification of new states received from the device.",
				Attributes: map[string]schema.Attribute{
					"pubsub_topic_name": schema.StringAttribute{
						Optional: true,
						// Computed:            true,
						MarkdownDescription: "A Cloud Pub/Sub topic name. For example, projects/myProject/topics/deviceEvents.",
					},
				},
			},
			"mqtt_config": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The configuration of MQTT for a device registry.",
				Attributes: map[string]schema.Attribute{
					"mqtt_enabled_state": schema.StringAttribute{
						MarkdownDescription: "If enabled, allows connections using the MQTT protocol. Otherwise, MQTT connections to this registry will fail.",
						Optional:            true,
						Computed:            true,
					},
				},
			},
			"http_config": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The configuration of the HTTP bridge for a device registry.",
				Attributes: map[string]schema.Attribute{
					"http_enabled_state": schema.StringAttribute{
						MarkdownDescription: "If enabled, allows devices to use DeviceService via the HTTP protocol. Otherwise, any requests to DeviceService will fail for this registry.",
						Optional:            true,
						Computed:            true,
					},
				},
			},
			"log_level": schema.StringAttribute{
				MarkdownDescription: "The default logging verbosity for activity from devices in this registry. The verbosity level can be overridden by Device.log_level.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"NONE",
						"ERROR",
						"INFO",
						"DEBUG",
						"",
					),
				},
			},
			"credentials": schema.ListNestedAttribute{
				// "credentials": schema.SetNestedAttribute{
				Optional:            true,
				MarkdownDescription: "List of public key certificates to authenticate devices.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"public_key_certificate": schema.SingleNestedAttribute{
							Required:    true,
							Description: "A public key certificate format and data.",
							Attributes: map[string]schema.Attribute{
								"format": schema.StringAttribute{
									Description: "The certificate format.",
									Required:    true,
								},
								"certificate": schema.StringAttribute{
									Description: "The certificate data.",
									Required:    true,
								},
								"x509_details": schema.SingleNestedAttribute{
									Optional:    true,
									Description: "Details of an X.509 certificate.",
									Attributes: map[string]schema.Attribute{
										"issuer": schema.StringAttribute{
											Description: "The entity that signed the certificate.",
											Optional:    true,
										},
										"subject": schema.StringAttribute{
											Description: "The entity the certificate and public key belong to.",
											Optional:    true,
										},
										"start_time": schema.StringAttribute{
											Description: "The time the certificate becomes valid.",
											Optional:    true,
										},
										"expiry_time": schema.StringAttribute{
											Description: "The time the certificate becomes invalid.",
											Optional:    true,
										},
										"signature_algorithm": schema.StringAttribute{
											Description: "The algorithm used to sign the certificate.",
											Optional:    true,
										},
										"public_key_type": schema.StringAttribute{
											Description: "The type of public key in the certificate.",
											Optional:    true,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *deviceRegistryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating iot device registry resource")

	var plan deviceRegistryResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 60*time.Minute)

	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Generate API request body from plan
	credentials := []*iot.RegistryCredential{}
	if len(plan.Credentials.Elements()) > 0 {

		var credentialsModel []CredentialsModel
		plan.Credentials.ElementsAs(ctx, &credentialsModel, false)
		for _, v := range credentialsModel {
			credentials = append(credentials, &iot.RegistryCredential{
				PublicKeyCertificate: &iot.PublicKeyCertificate{
					Format:      v.PublicKeyCertificate.Format.ValueString(),
					Certificate: v.PublicKeyCertificate.Certificate.ValueString(),
					X509Details: &iot.X509CertificateDetails{
						Issuer:             v.PublicKeyCertificate.X509Details.Issuer.ValueString(),
						Subject:            v.PublicKeyCertificate.X509Details.Subject.ValueString(),
						StartTime:          v.PublicKeyCertificate.X509Details.StartTime.ValueString(),
						ExpiryTime:         v.PublicKeyCertificate.X509Details.ExpiryTime.ValueString(),
						SignatureAlgorithm: v.PublicKeyCertificate.X509Details.SignatureAlgorithm.ValueString(),
						PublicKeyType:      v.PublicKeyCertificate.X509Details.PublicKeyType.ValueString(),
					},
				},
			})
		}
	}

	event_notification_configs := []*iot.EventNotificationConfig{}
	for _, v := range plan.EventNotificationConfigs {
		// Support both SubfolderMatches and SubFolderMatches
		if !v.SubFolderMatches.IsNull() && v.SubFolderMatches.ValueString() != "" {
			v.SubfolderMatches = v.SubFolderMatches
		} else {
			v.SubFolderMatches = v.SubfolderMatches
		}
		event_notification_configs = append(event_notification_configs, &iot.EventNotificationConfig{
			PubsubTopicName:  v.PubsubTopicName.ValueString(),
			SubfolderMatches: v.SubfolderMatches.ValueString(),
		})
	}

	var stateNotificationConfig StateNotificationConfigModel
	plan.StateNotificationConfig.As(ctx, &stateNotificationConfig, basetypes.ObjectAsOptions{})
	var mqttConfig MqttConfigModel
	plan.MqttConfig.As(ctx, &mqttConfig, basetypes.ObjectAsOptions{})
	var httpConfig HttpConfigModel
	plan.HttpConfig.As(ctx, &httpConfig, basetypes.ObjectAsOptions{})

	createRequestPayload := iot.DeviceRegistry{
		Id:                       plan.ID.ValueString(),
		Name:                     plan.Name.ValueString(),
		LogLevel:                 plan.LogLevel.ValueString(),
		EventNotificationConfigs: event_notification_configs,
		Credentials:              credentials,
		StateNotificationConfig: &iot.StateNotificationConfig{
			PubsubTopicName: stateNotificationConfig.PubsubTopicName.ValueString(),
		},
		MqttConfig: &iot.MqttConfig{
			MqttEnabledState: mqttConfig.MqttEnabledState.ValueString(),
		},
		HttpConfig: &iot.HttpConfig{
			HttpEnabledState: httpConfig.HttpEnabledState.ValueString(),
		},
	}

	payloadString := fmt.Sprintf("%+v", createRequestPayload)
	ctx = tflog.SetField(ctx, "create payload in CREATE", payloadString)

	// Create a new device registry resource on ClearBlade IoT Core
	parent := fmt.Sprintf("projects/%s/locations/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"))
	registry, err := r.client.Projects.Locations.Registries.Create(parent, &createRequestPayload).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating a device registry",
			"Could not create a new device registry, unexpected error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Created a device registry")

	// Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(registry.Name)
	plan.LogLevel = types.StringValue(registry.LogLevel)

	plan.EventNotificationConfigs = []EventNotificationConfigsModel{}
	for _, eventNotificationConfig := range registry.EventNotificationConfigs {
		plan.EventNotificationConfigs = append(plan.EventNotificationConfigs, EventNotificationConfigsModel{
			PubsubTopicName:  types.StringValue(eventNotificationConfig.PubsubTopicName),
			SubfolderMatches: types.StringValue(eventNotificationConfig.SubfolderMatches),
			SubFolderMatches: types.StringValue(eventNotificationConfig.SubfolderMatches),
		})
	}

	if plan.StateNotificationConfig.IsNull() {
		attributes := map[string]attr.Value{
			"pubsub_topic_name": types.StringNull(),
		}
		plan.StateNotificationConfig = types.ObjectValueMust(StateNotificationConfigModelTypes, attributes)
	} else {
		ctx = tflog.SetField(ctx, "processing pubsub topic", registry.StateNotificationConfig.PubsubTopicName)
		tflog.Info(ctx, "processing pubsub")
		attributes := map[string]attr.Value{
			"pubsub_topic_name": types.StringValue(registry.StateNotificationConfig.PubsubTopicName),
		}
		plan.StateNotificationConfig = types.ObjectValueMust(StateNotificationConfigModelTypes, attributes)
	}

	if plan.MqttConfig.IsNull() {
		attributes := map[string]attr.Value{
			"mqtt_enabled_state": types.StringNull(),
		}
		plan.MqttConfig = types.ObjectValueMust(MqttConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"mqtt_enabled_state": types.StringValue(registry.MqttConfig.MqttEnabledState),
		}
		plan.MqttConfig = types.ObjectValueMust(MqttConfigModelTypes, attributes)
	}

	if plan.HttpConfig.IsNull() {
		attributes := map[string]attr.Value{
			"http_enabled_state": types.StringNull(),
		}
		plan.HttpConfig = types.ObjectValueMust(HttpConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"http_enabled_state": types.StringValue(registry.HttpConfig.HttpEnabledState),
		}
		plan.HttpConfig = types.ObjectValueMust(HttpConfigModelTypes, attributes)
	}

	if plan.Credentials.IsNull() {
		tflog.Debug(ctx, "value detected NULL - CREATE")
		plan.Credentials = types.ListNull(plan.Credentials.ElementType(ctx))
		// plan.Credentials = types.SetNull(plan.Credentials.ElementType(ctx))
	}

	if plan.Credentials.IsUnknown() {
		tflog.Debug(ctx, "value detected UNKNOWN - CREATE")

		registryString := fmt.Sprintf("%+v", registry.Credentials)
		ctx = tflog.SetField(ctx, "registryString - CREATE", registryString)

		var credentials []CredentialsModel

		for _, credential := range registry.Credentials {
			m := CredentialsModel{
				PublicKeyCertificate: PublicKeyCertificateModel{
					Format:      types.StringValue(credential.PublicKeyCertificate.Format),
					Certificate: types.StringValue(credential.PublicKeyCertificate.Certificate),
					X509Details: X509CertificateDetailsModel{
						Issuer:             types.StringValue(credential.PublicKeyCertificate.X509Details.Issuer),
						Subject:            types.StringValue(credential.PublicKeyCertificate.X509Details.Subject),
						StartTime:          types.StringValue(credential.PublicKeyCertificate.X509Details.StartTime),
						ExpiryTime:         types.StringValue(credential.PublicKeyCertificate.X509Details.ExpiryTime),
						SignatureAlgorithm: types.StringValue(credential.PublicKeyCertificate.X509Details.SignatureAlgorithm),
						PublicKeyType:      types.StringValue(credential.PublicKeyCertificate.X509Details.PublicKeyType),
					},
				},
			}
			credentials = append(credentials, m)
		}
		plan.Credentials, _ = types.ListValueFrom(ctx, plan.Credentials.ElementType(ctx), credentials)
		// plan.Credentials, _ = types.SetValueFrom(ctx, plan.Credentials.ElementType(ctx), credentials)
	}

	if !plan.Credentials.IsUnknown() && !plan.Credentials.IsNull() {
		tflog.Debug(ctx, "value detected KNOWN - CREATE")

		registryString := fmt.Sprintf("%+v", len(registry.Credentials))
		ctx = tflog.SetField(ctx, "length registryString - CREATE", registryString)

		var credentials []CredentialsModel

		for _, credential := range registry.Credentials {
			m := CredentialsModel{
				PublicKeyCertificate: PublicKeyCertificateModel{
					Format:      types.StringValue(credential.PublicKeyCertificate.Format),
					Certificate: types.StringValue(credential.PublicKeyCertificate.Certificate),
					X509Details: X509CertificateDetailsModel{
						Issuer:             types.StringValue(credential.PublicKeyCertificate.X509Details.Issuer),
						Subject:            types.StringValue(credential.PublicKeyCertificate.X509Details.Subject),
						StartTime:          types.StringValue(credential.PublicKeyCertificate.X509Details.StartTime),
						ExpiryTime:         types.StringValue(credential.PublicKeyCertificate.X509Details.ExpiryTime),
						SignatureAlgorithm: types.StringValue(credential.PublicKeyCertificate.X509Details.SignatureAlgorithm),
						PublicKeyType:      types.StringValue(credential.PublicKeyCertificate.X509Details.PublicKeyType),
					},
				},
			}
			credentials = append(credentials, m)
		}
		if credentials == nil {
			tflog.Debug(ctx, "nil value detected - CREATE")
			// resp.Diagnostics.AddError(
			// 	"Error",
			// 	"Null list was set in the template",
			// )
			// return
		} else {
			plan.Credentials, _ = types.ListValueFrom(ctx, plan.Credentials.ElementType(ctx), credentials)
			// plan.Credentials, _ = types.SetValueFrom(ctx, plan.Credentials.ElementType(ctx), credentials)
		}

	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information and refreshes the Terraform state with the latest data.
func (r *deviceRegistryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading the device registry resource")

	// Get current state
	var state deviceRegistryResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed registry value from ClearBlade IoT Core
	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"), state.ID.ValueString())
	registry, err := r.client.Projects.Locations.Registries.Get(parent).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading ClearBlade IoT Core Registry",
			"Could not read ClearBlade IoT Core registry ID "+state.Name.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite items with refreshed state
	state.Name = types.StringValue(registry.Name)

	state.LogLevel = types.StringValue(registry.LogLevel)

	state.EventNotificationConfigs = []EventNotificationConfigsModel{}
	for _, eventNotificationConfig := range registry.EventNotificationConfigs {
		state.EventNotificationConfigs = append(state.EventNotificationConfigs, EventNotificationConfigsModel{
			PubsubTopicName:  types.StringValue(eventNotificationConfig.PubsubTopicName),
			SubfolderMatches: types.StringValue(eventNotificationConfig.SubfolderMatches),
		})
	}

	state.MqttConfig = types.ObjectValueMust(MqttConfigModelTypes, map[string]attr.Value{
		"mqtt_enabled_state": types.StringValue(registry.MqttConfig.MqttEnabledState),
	})

	state.StateNotificationConfig = types.ObjectValueMust(StateNotificationConfigModelTypes, map[string]attr.Value{
		"pubsub_topic_name": types.StringValue(registry.StateNotificationConfig.PubsubTopicName),
	})

	state.HttpConfig = types.ObjectValueMust(HttpConfigModelTypes, map[string]attr.Value{
		"http_enabled_state": types.StringValue(registry.HttpConfig.HttpEnabledState),
	})

	if state.Credentials.IsNull() {
		tflog.Debug(ctx, "value detected NULL - READ")
		state.Credentials = types.ListNull(state.Credentials.ElementType(ctx))
		// state.Credentials = types.SetNull(state.Credentials.ElementType(ctx))
	} else {
		tflog.Debug(ctx, "value detected KNOWN - READ")
		var credentials []CredentialsModel

		for _, credential := range registry.Credentials {
			m := CredentialsModel{
				PublicKeyCertificate: PublicKeyCertificateModel{
					Format:      types.StringValue(credential.PublicKeyCertificate.Format),
					Certificate: types.StringValue(credential.PublicKeyCertificate.Certificate),
					X509Details: X509CertificateDetailsModel{
						Issuer:             types.StringValue(credential.PublicKeyCertificate.X509Details.Issuer),
						Subject:            types.StringValue(credential.PublicKeyCertificate.X509Details.Subject),
						StartTime:          types.StringValue(credential.PublicKeyCertificate.X509Details.StartTime),
						ExpiryTime:         types.StringValue(credential.PublicKeyCertificate.X509Details.ExpiryTime),
						SignatureAlgorithm: types.StringValue(credential.PublicKeyCertificate.X509Details.SignatureAlgorithm),
						PublicKeyType:      types.StringValue(credential.PublicKeyCertificate.X509Details.PublicKeyType),
					},
				},
			}
			credentials = append(credentials, m)
		}
		state.Credentials, _ = types.ListValueFrom(ctx, state.Credentials.ElementType(ctx), credentials)
		// state.Credentials, _ = types.SetValueFrom(ctx, state.Credentials.ElementType(ctx), credentials)
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

// Update updates the resource and sets the updated Terraform state on success.
func (r *deviceRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Debug(ctx, "Updating iot device registry resource")

	// Retrieve values from plan
	var plan deviceRegistryResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	credentials := []*iot.RegistryCredential{}
	if len(plan.Credentials.Elements()) > 0 {
		var credentialsModel []CredentialsModel
		plan.Credentials.ElementsAs(ctx, &credentialsModel, false)

		for _, v := range credentialsModel {
			credentials = append(credentials, &iot.RegistryCredential{
				PublicKeyCertificate: &iot.PublicKeyCertificate{
					Format:      v.PublicKeyCertificate.Format.ValueString(),
					Certificate: v.PublicKeyCertificate.Certificate.ValueString(),
					X509Details: &iot.X509CertificateDetails{
						Issuer:             v.PublicKeyCertificate.X509Details.Issuer.ValueString(),
						Subject:            v.PublicKeyCertificate.X509Details.Subject.ValueString(),
						StartTime:          v.PublicKeyCertificate.X509Details.StartTime.ValueString(),
						ExpiryTime:         v.PublicKeyCertificate.X509Details.ExpiryTime.ValueString(),
						SignatureAlgorithm: v.PublicKeyCertificate.X509Details.SignatureAlgorithm.ValueString(),
						PublicKeyType:      v.PublicKeyCertificate.X509Details.PublicKeyType.ValueString(),
					},
				},
			})
		}
	}

	eventNotificationConfigs := []*iot.EventNotificationConfig{}
	for _, v := range plan.EventNotificationConfigs {
		// Support both SubfolderMatches and SubFolderMatches
		if !v.SubFolderMatches.IsNull() && v.SubFolderMatches.ValueString() != "" {
			v.SubfolderMatches = v.SubFolderMatches
		} else {
			v.SubFolderMatches = v.SubfolderMatches
		}
		tflog.Debug(ctx, "registry update event 1")
		ctx = tflog.SetField(ctx, "registry update event 1 notify pubsub topic", v.PubsubTopicName.ValueString())
		ctx = tflog.SetField(ctx, "registry update event 1 notify subfolder matches", v.SubfolderMatches.ValueString())
		eventNotificationConfigs = append(eventNotificationConfigs, &iot.EventNotificationConfig{
			PubsubTopicName:  v.PubsubTopicName.ValueString(),
			SubfolderMatches: v.SubfolderMatches.ValueString(),
		})
	}

	var stateNotificationConfig StateNotificationConfigModel
	plan.StateNotificationConfig.As(ctx, &stateNotificationConfig, basetypes.ObjectAsOptions{})
	var mqttConfig MqttConfigModel
	plan.MqttConfig.As(ctx, &mqttConfig, basetypes.ObjectAsOptions{})
	var httpConfig HttpConfigModel
	plan.HttpConfig.As(ctx, &httpConfig, basetypes.ObjectAsOptions{})

	updateRequestPayload := iot.DeviceRegistry{
		EventNotificationConfigs: eventNotificationConfigs,
		Credentials:              credentials,
		Id:                       plan.ID.ValueString(),
		Name:                     plan.Name.ValueString(),
		StateNotificationConfig: &iot.StateNotificationConfig{
			PubsubTopicName: stateNotificationConfig.PubsubTopicName.ValueString(),
		},
		MqttConfig: &iot.MqttConfig{
			MqttEnabledState: mqttConfig.MqttEnabledState.ValueString(),
		},
		HttpConfig: &iot.HttpConfig{
			HttpEnabledState: httpConfig.HttpEnabledState.ValueString(),
		},
		LogLevel: plan.LogLevel.ValueString(),
	}

	payloadString := fmt.Sprintf("%+v", updateRequestPayload)
	ctx = tflog.SetField(ctx, "create payload in UPDATE", payloadString)

	// Update an existing registry
	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"), plan.ID.ValueString())

	_, err := r.client.Projects.Locations.Registries.
		Patch(parent, &updateRequestPayload).
		UpdateMask(`httpConfig.http_enabled_state,logLevel,mqttConfig.mqtt_enabled_state,stateNotificationConfig.pubsub_topic_name,credentials,eventNotificationConfigs`).Do()
	// ["eventNotificationConfigs","stateNotificationConfig.pubsub_topic_name","mqttConfig.mqtt_enabled_state","httpConfig.http_enabled_state","logLevel","credentials"]

	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating a device registry",
			"Could not update device registry, unexpected error: "+err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "device registry updated")

	// Fetch updated registry value from ClearBlade IoT Core
	registry, err := r.client.Projects.Locations.Registries.Get(parent).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading ClearBlade IoT Core Registry",
			"Could not read ClearBlade IoT Core registry ID "+plan.Name.ValueString()+": "+err.Error(),
		)
		return
	}

	// Update registry resource - Map response body to schema and populate Computed attribute values
	plan.Name = types.StringValue(registry.Name)
	plan.LogLevel = types.StringValue(registry.LogLevel)

	if plan.StateNotificationConfig.IsNull() {
		attributes := map[string]attr.Value{
			"pubsub_topic_name": types.StringNull(),
		}
		plan.StateNotificationConfig = types.ObjectValueMust(StateNotificationConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"pubsub_topic_name": types.StringValue(registry.StateNotificationConfig.PubsubTopicName),
		}
		plan.StateNotificationConfig = types.ObjectValueMust(StateNotificationConfigModelTypes, attributes)
	}

	if plan.MqttConfig.IsNull() {
		attributes := map[string]attr.Value{
			"mqtt_enabled_state": types.StringNull(),
		}
		plan.MqttConfig = types.ObjectValueMust(MqttConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"mqtt_enabled_state": types.StringValue(registry.MqttConfig.MqttEnabledState),
		}
		plan.MqttConfig = types.ObjectValueMust(MqttConfigModelTypes, attributes)
	}

	if plan.HttpConfig.IsNull() {
		attributes := map[string]attr.Value{
			"http_enabled_state": types.StringNull(),
		}
		plan.HttpConfig = types.ObjectValueMust(HttpConfigModelTypes, attributes)
	} else {
		attributes := map[string]attr.Value{
			"http_enabled_state": types.StringValue(registry.HttpConfig.HttpEnabledState),
		}
		plan.HttpConfig = types.ObjectValueMust(HttpConfigModelTypes, attributes)
	}

	if plan.EventNotificationConfigs == nil || (reflect.ValueOf(plan.EventNotificationConfigs).Kind() == reflect.Ptr && reflect.ValueOf(plan.EventNotificationConfigs).IsNil()) {

	} else {

		plan.EventNotificationConfigs = []EventNotificationConfigsModel{}
		for _, eventNotificationConfig := range registry.EventNotificationConfigs {
			plan.EventNotificationConfigs = append(plan.EventNotificationConfigs, EventNotificationConfigsModel{
				PubsubTopicName:  types.StringValue(eventNotificationConfig.PubsubTopicName),
				SubfolderMatches: types.StringValue(eventNotificationConfig.SubfolderMatches),
				SubFolderMatches: types.StringValue(eventNotificationConfig.SubfolderMatches),
			})
		}
	}

	if plan.Credentials.IsNull() {
		tflog.Debug(ctx, "value detected NULL - UPDATE")
		plan.Credentials = types.ListNull(plan.Credentials.ElementType(ctx))
		// plan.Credentials = types.SetNull(plan.Credentials.ElementType(ctx))
	}

	if plan.Credentials.IsUnknown() {
		tflog.Debug(ctx, "value detected UNKNOWN - UPDATE")
		var credentials []CredentialsModel

		for _, credential := range registry.Credentials {
			m := CredentialsModel{
				PublicKeyCertificate: PublicKeyCertificateModel{
					Format:      types.StringValue(credential.PublicKeyCertificate.Format),
					Certificate: types.StringValue(credential.PublicKeyCertificate.Certificate),
					X509Details: X509CertificateDetailsModel{
						Issuer:             types.StringValue(credential.PublicKeyCertificate.X509Details.Issuer),
						Subject:            types.StringValue(credential.PublicKeyCertificate.X509Details.Subject),
						StartTime:          types.StringValue(credential.PublicKeyCertificate.X509Details.StartTime),
						ExpiryTime:         types.StringValue(credential.PublicKeyCertificate.X509Details.ExpiryTime),
						SignatureAlgorithm: types.StringValue(credential.PublicKeyCertificate.X509Details.SignatureAlgorithm),
						PublicKeyType:      types.StringValue(credential.PublicKeyCertificate.X509Details.PublicKeyType),
					},
				},
			}
			credentials = append(credentials, m)
		}
		plan.Credentials, _ = types.ListValueFrom(ctx, plan.Credentials.ElementType(ctx), credentials)
		// plan.Credentials, _ = types.SetValueFrom(ctx, plan.Credentials.ElementType(ctx), credentials)
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

// Delete deletes the resource and removes the Terraform state on success.
func (r *deviceRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Deleting a device registry resource")

	// Retrieve values from state
	var state deviceRegistryResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing registry on ClearBlade IoT Core
	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s", os.Getenv("CLEARBLADE_PROJECT"), os.Getenv("CLEARBLADE_REGION"), state.ID.ValueString())
	_, err := r.client.Projects.Locations.Registries.Delete(parent).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting ClearBlade IoT Core Registry",
			"Could not delete Registry, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *deviceRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	tflog.Debug(ctx, "registry import event")
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Metadata returns the data source type name.
func (r *deviceRegistryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iot_registry"
}

// Configure adds the provider configured client to the data source.
func (r *deviceRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

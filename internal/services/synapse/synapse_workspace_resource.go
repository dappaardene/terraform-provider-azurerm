package synapse

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/synapse/mgmt/2021-03-01/synapse"
	"github.com/gofrs/uuid"
	"github.com/hashicorp/go-azure-helpers/response"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/location"
	keyVaultValidate "github.com/hashicorp/terraform-provider-azurerm/internal/services/keyvault/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/synapse/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/synapse/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tags"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

const (
	workspaceVSTSConfiguration   = "WorkspaceVSTSConfiguration"
	workspaceGitHubConfiguration = "WorkspaceGitHubConfiguration"
)

func resourceSynapseWorkspace() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceSynapseWorkspaceCreate,
		Read:   resourceSynapseWorkspaceRead,
		Update: resourceSynapseWorkspaceUpdate,
		Delete: resourceSynapseWorkspaceDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := parse.WorkspaceID(id)
			return err
		}),

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.WorkspaceName,
			},

			"resource_group_name": azure.SchemaResourceGroupName(),

			"location": azure.SchemaLocation(),

			"storage_data_lake_gen2_filesystem_id": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
			},

			"sql_administrator_login": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validate.SqlAdministratorLoginName,
			},

			"sql_administrator_login_password": {
				Type:      pluginsdk.TypeString,
				Required:  true,
				Sensitive: true,
			},

			"data_exfiltration_protection_enabled": {
				Type:     pluginsdk.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"managed_virtual_network_enabled": {
				Type:     pluginsdk.TypeBool,
				Optional: true,
				ForceNew: true,
			},

			"aad_admin": {
				Type:       pluginsdk.TypeList,
				Optional:   true,
				Computed:   true,
				MaxItems:   1,
				ConfigMode: pluginsdk.SchemaConfigModeAttr,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"login": {
							Type:     pluginsdk.TypeString,
							Required: true,
						},

						"object_id": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.IsUUID,
						},

						"tenant_id": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.IsUUID,
						},
					},
				},
			},

			"connectivity_endpoints": {
				Type:     pluginsdk.TypeMap,
				Computed: true,
				Elem: &pluginsdk.Schema{
					Type: pluginsdk.TypeString,
				},
			},

			"identity": {
				Type:     pluginsdk.TypeList,
				Computed: true,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"type": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},

						"principal_id": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},

						"tenant_id": {
							Type:     pluginsdk.TypeString,
							Computed: true,
						},
					},
				},
			},

			"managed_resource_group_name": azure.SchemaResourceGroupNameOptionalComputed(),

			"azure_devops_repo": {
				Type:          pluginsdk.TypeList,
				Optional:      true,
				MaxItems:      1,
				ConflictsWith: []string{"github_repo"},
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"account_name": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"branch_name": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"project_name": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"repository_name": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"root_folder": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validate.RepoRootFolder(),
						},
						"tenant_id": {
							Type:         pluginsdk.TypeString,
							Optional:     true,
							ValidateFunc: validation.IsUUID,
						},
					},
				},
			},

			"github_repo": {
				Type:          pluginsdk.TypeList,
				Optional:      true,
				MaxItems:      1,
				ConflictsWith: []string{"azure_devops_repo"},
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"account_name": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"branch_name": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"git_url": {
							Type:         pluginsdk.TypeString,
							Optional:     true,
							ValidateFunc: validation.IsURLWithHTTPS,
						},
						"repository_name": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"root_folder": {
							Type:         pluginsdk.TypeString,
							Required:     true,
							ValidateFunc: validate.RepoRootFolder(),
						},
					},
				},
			},

			"sql_identity_control_enabled": {
				Type:     pluginsdk.TypeBool,
				Optional: true,
			},

			"customer_managed_key_versionless_id": {
				Type:         pluginsdk.TypeString,
				Optional:     true,
				ValidateFunc: keyVaultValidate.VersionlessNestedItemId,
			},

			"tags": tags.Schema(),
		},
	}
}

func resourceSynapseWorkspaceCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Synapse.WorkspaceClient
	aadAdminClient := meta.(*clients.Client).Synapse.WorkspaceAadAdminsClient
	identitySQLControlClient := meta.(*clients.Client).Synapse.WorkspaceManagedIdentitySQLControlSettingsClient
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	name := d.Get("name").(string)
	resourceGroup := d.Get("resource_group_name").(string)

	existing, err := client.Get(ctx, resourceGroup, name)
	if err != nil {
		if !utils.ResponseWasNotFound(existing.Response) {
			return fmt.Errorf("checking for present of existing Synapse Workspace %q (Resource Group %q): %+v", name, resourceGroup, err)
		}
	}
	if existing.ID != nil && *existing.ID != "" {
		return tf.ImportAsExistsError("azurerm_synapse_workspace", *existing.ID)
	}

	managedVirtualNetwork := ""
	if d.Get("managed_virtual_network_enabled").(bool) {
		managedVirtualNetwork = "default"
	}

	workspaceInfo := synapse.Workspace{
		Location: utils.String(location.Normalize(d.Get("location").(string))),
		WorkspaceProperties: &synapse.WorkspaceProperties{
			DefaultDataLakeStorage:           expandArmWorkspaceDataLakeStorageAccountDetails(d.Get("storage_data_lake_gen2_filesystem_id").(string)),
			ManagedVirtualNetwork:            utils.String(managedVirtualNetwork),
			SQLAdministratorLogin:            utils.String(d.Get("sql_administrator_login").(string)),
			SQLAdministratorLoginPassword:    utils.String(d.Get("sql_administrator_login_password").(string)),
			ManagedResourceGroupName:         utils.String(d.Get("managed_resource_group_name").(string)),
			WorkspaceRepositoryConfiguration: expandWorkspaceRepositoryConfiguration(d),
			Encryption:                       expandEncryptionDetails(d),
		},
		Identity: &synapse.ManagedIdentity{
			Type: synapse.ResourceIdentityTypeSystemAssigned,
		},
		Tags: tags.Expand(d.Get("tags").(map[string]interface{})),
	}

	dataExfiltrationProtectionEnabled := d.Get("data_exfiltration_protection_enabled").(bool)

	if dataExfiltrationProtectionEnabled {
		workspaceInfo.ManagedVirtualNetworkSettings = &synapse.ManagedVirtualNetworkSettings{
			PreventDataExfiltration: utils.Bool(dataExfiltrationProtectionEnabled),
		}
	}

	future, err := client.CreateOrUpdate(ctx, resourceGroup, name, workspaceInfo)
	if err != nil {
		return fmt.Errorf("creating Synapse Workspace %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting on creation for Synapse Workspace %q (Resource Group %q): %+v", name, resourceGroup, err)
	}

	aadAdmin := expandArmWorkspaceAadAdmin(d.Get("aad_admin").([]interface{}))
	if aadAdmin != nil {
		workspaceAadAdminsCreateOrUpdateFuture, err := aadAdminClient.CreateOrUpdate(ctx, resourceGroup, name, *aadAdmin)
		if err != nil {
			return fmt.Errorf("updating Synapse Workspace %q Sql Admin (Resource Group %q): %+v", name, resourceGroup, err)
		}

		if err = workspaceAadAdminsCreateOrUpdateFuture.WaitForCompletionRef(ctx, client.Client); err != nil {
			return fmt.Errorf("waiting on updating for Synapse Workspace %q Sql Admin (Resource Group %q): %+v", name, resourceGroup, err)
		}
	}

	sqlControlSettings := expandIdentityControlSQLSettings(d.Get("sql_identity_control_enabled").(bool))
	if _, err = identitySQLControlClient.CreateOrUpdate(ctx, resourceGroup, name, *sqlControlSettings); err != nil {
		return fmt.Errorf("Granting workspace identity control for SQL pool: %+v", err)
	}

	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	id := parse.NewWorkspaceID(subscriptionId, resourceGroup, name)
	d.SetId(id.ID())

	return resourceSynapseWorkspaceRead(d, meta)
}

func resourceSynapseWorkspaceRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Synapse.WorkspaceClient
	aadAdminClient := meta.(*clients.Client).Synapse.WorkspaceAadAdminsClient
	identitySQLControlClient := meta.(*clients.Client).Synapse.WorkspaceManagedIdentitySQLControlSettingsClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.WorkspaceID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			log.Printf("[INFO] synapse %q does not exist - removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("retrieving Synapse Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	aadAdmin, err := aadAdminClient.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		if !utils.ResponseWasNotFound(aadAdmin.Response) {
			return fmt.Errorf("retrieving Synapse Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
		}
	}

	sqlControlSettings, err := identitySQLControlClient.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		return fmt.Errorf("retrieving workspace identity control for SQL pool: %+v", err)
	}

	d.Set("name", id.Name)
	d.Set("resource_group_name", id.ResourceGroup)
	d.Set("location", location.NormalizeNilable(resp.Location))
	if err := d.Set("identity", flattenArmWorkspaceManagedIdentity(resp.Identity)); err != nil {
		return fmt.Errorf("setting `identity`: %+v", err)
	}
	if props := resp.WorkspaceProperties; props != nil {
		managedVirtualNetworkEnabled := false
		if props.ManagedVirtualNetwork != nil && strings.EqualFold(*props.ManagedVirtualNetwork, "default") {
			managedVirtualNetworkEnabled = true
			if props.ManagedVirtualNetworkSettings != nil {
				d.Set("data_exfiltration_protection_enabled", props.ManagedVirtualNetworkSettings.PreventDataExfiltration)
			}
		}
		d.Set("managed_virtual_network_enabled", managedVirtualNetworkEnabled)
		d.Set("storage_data_lake_gen2_filesystem_id", flattenArmWorkspaceDataLakeStorageAccountDetails(props.DefaultDataLakeStorage))
		d.Set("sql_administrator_login", props.SQLAdministratorLogin)
		d.Set("managed_resource_group_name", props.ManagedResourceGroupName)
		d.Set("connectivity_endpoints", utils.FlattenMapStringPtrString(props.ConnectivityEndpoints))
		d.Set("customer_managed_key_versionless_id", flattenEncryptionDetails(props.Encryption))

		repoType, repo := flattenWorkspaceRepositoryConfiguration(props.WorkspaceRepositoryConfiguration)
		if repoType == workspaceVSTSConfiguration {
			if err := d.Set("azure_devops_repo", repo); err != nil {
				return fmt.Errorf("setting `azure_devops_repo`: %+v", err)
			}
		} else if repoType == workspaceGitHubConfiguration {
			if err := d.Set("github_repo", repo); err != nil {
				return fmt.Errorf("setting `github_repo`: %+v", err)
			}
		}
	}
	if err := d.Set("aad_admin", flattenArmWorkspaceAadAdmin(aadAdmin.AadAdminProperties)); err != nil {
		return fmt.Errorf("setting `aad_admin`: %+v", err)
	}
	if err := d.Set("sql_identity_control_enabled", flattenIdentityControlSQLSettings(sqlControlSettings)); err != nil {
		return fmt.Errorf("setting `sql_identity_control_enabled`: %+v", err)
	}

	return tags.FlattenAndSet(d, resp.Tags)
}

func resourceSynapseWorkspaceUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Synapse.WorkspaceClient
	aadAdminClient := meta.(*clients.Client).Synapse.WorkspaceAadAdminsClient
	identitySQLControlClient := meta.(*clients.Client).Synapse.WorkspaceManagedIdentitySQLControlSettingsClient
	ctx, cancel := timeouts.ForUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.WorkspaceID(d.Id())
	if err != nil {
		return err
	}

	if d.HasChanges("tags", "sql_administrator_login_password", "github_repo", "azure_devops_repo", "customer_managed_key_versionless_id") {
		workspacePatchInfo := synapse.WorkspacePatchInfo{
			Tags: tags.Expand(d.Get("tags").(map[string]interface{})),
			WorkspacePatchProperties: &synapse.WorkspacePatchProperties{
				SQLAdministratorLoginPassword:    utils.String(d.Get("sql_administrator_login_password").(string)),
				WorkspaceRepositoryConfiguration: expandWorkspaceRepositoryConfiguration(d),
				Encryption:                       expandEncryptionDetails(d),
			},
		}

		future, err := client.Update(ctx, id.ResourceGroup, id.Name, workspacePatchInfo)
		if err != nil {
			return fmt.Errorf("updating Synapse Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
		}

		if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
			return fmt.Errorf("waiting on updating future for Synapse Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
		}
	}

	if d.HasChange("aad_admin") {
		aadAdmin := expandArmWorkspaceAadAdmin(d.Get("aad_admin").([]interface{}))
		if aadAdmin != nil {
			workspaceAadAdminsCreateOrUpdateFuture, err := aadAdminClient.CreateOrUpdate(ctx, id.ResourceGroup, id.Name, *aadAdmin)
			if err != nil {
				return fmt.Errorf("updating Synapse Workspace %q Sql Admin (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
			}

			if err = workspaceAadAdminsCreateOrUpdateFuture.WaitForCompletionRef(ctx, client.Client); err != nil {
				return fmt.Errorf("waiting on updating for Synapse Workspace %q Sql Admin (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
			}
		} else {
			workspaceAadAdminsDeleteFuture, err := aadAdminClient.Delete(ctx, id.ResourceGroup, id.Name)
			if err != nil {
				return fmt.Errorf("setting empty Synapse Workspace %q Sql Admin (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
			}

			if err = workspaceAadAdminsDeleteFuture.WaitForCompletionRef(ctx, client.Client); err != nil {
				return fmt.Errorf("waiting on setting empty Synapse Workspace %q Sql Admin (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
			}
		}
	}

	if d.HasChange("sql_identity_control_enabled") {
		sqlControlSettings := expandIdentityControlSQLSettings(d.Get("sql_identity_control_enabled").(bool))
		if _, err = identitySQLControlClient.CreateOrUpdate(ctx, id.ResourceGroup, id.Name, *sqlControlSettings); err != nil {
			return fmt.Errorf("Updating workspace identity control for SQL pool: %+v", err)
		}
	}

	return resourceSynapseWorkspaceRead(d, meta)
}

func resourceSynapseWorkspaceDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Synapse.WorkspaceClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.WorkspaceID(d.Id())
	if err != nil {
		return err
	}

	future, err := client.Delete(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		return fmt.Errorf("deleting Synapse Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	// sometimes the waitForCompletion rest api will return 404
	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		if !response.WasNotFound(future.Response()) {
			return fmt.Errorf("waiting for Synapse Workspace %q (Resource Group %q) to be deleted: %+v", id.Name, id.ResourceGroup, err)
		}
	}

	return nil
}

func expandArmWorkspaceDataLakeStorageAccountDetails(storageDataLakeGen2FilesystemId string) *synapse.DataLakeStorageAccountDetails {
	uri, _ := url.Parse(storageDataLakeGen2FilesystemId)
	return &synapse.DataLakeStorageAccountDetails{
		AccountURL: utils.String(fmt.Sprintf("%s://%s", uri.Scheme, uri.Host)), // https://storageaccountname.dfs.core.windows.net/filesystemname -> https://storageaccountname.dfs.core.windows.net
		Filesystem: utils.String(uri.Path[1:]),                                 // https://storageaccountname.dfs.core.windows.net/filesystemname -> filesystemname
	}
}

func expandArmWorkspaceAadAdmin(input []interface{}) *synapse.WorkspaceAadAdminInfo {
	if len(input) == 0 || input[0] == nil {
		return nil
	}
	v := input[0].(map[string]interface{})
	return &synapse.WorkspaceAadAdminInfo{
		AadAdminProperties: &synapse.AadAdminProperties{
			TenantID:          utils.String(v["tenant_id"].(string)),
			Login:             utils.String(v["login"].(string)),
			AdministratorType: utils.String("ActiveDirectory"),
			Sid:               utils.String(v["object_id"].(string)),
		},
	}
}

func expandWorkspaceRepositoryConfiguration(d *pluginsdk.ResourceData) *synapse.WorkspaceRepositoryConfiguration {
	if azdoList, ok := d.GetOk("azure_devops_repo"); ok {
		azdo := azdoList.([]interface{})[0].(map[string]interface{})
		config := synapse.WorkspaceRepositoryConfiguration{
			Type:                utils.String(workspaceVSTSConfiguration),
			AccountName:         utils.String(azdo["account_name"].(string)),
			CollaborationBranch: utils.String(azdo["branch_name"].(string)),
			ProjectName:         utils.String(azdo["project_name"].(string)),
			RepositoryName:      utils.String(azdo["repository_name"].(string)),
			RootFolder:          utils.String(azdo["root_folder"].(string)),
		}
		if azdoTenantId := uuid.FromStringOrNil(azdo["tenant_id"].(string)); azdoTenantId != uuid.Nil {
			config.TenantID = &azdoTenantId
		}
		return &config
	}

	if githubList, ok := d.GetOk("github_repo"); ok {
		github := githubList.([]interface{})[0].(map[string]interface{})
		return &synapse.WorkspaceRepositoryConfiguration{
			Type:                utils.String(workspaceGitHubConfiguration),
			AccountName:         utils.String(github["account_name"].(string)),
			CollaborationBranch: utils.String(github["branch_name"].(string)),
			HostName:            utils.String(github["git_url"].(string)),
			RepositoryName:      utils.String(github["repository_name"].(string)),
			RootFolder:          utils.String(github["root_folder"].(string)),
		}
	}

	return nil
}

func expandIdentityControlSQLSettings(enabled bool) *synapse.ManagedIdentitySQLControlSettingsModel {
	var desiredState synapse.DesiredState
	if enabled {
		desiredState = synapse.DesiredStateEnabled
	} else {
		desiredState = synapse.DesiredStateDisabled
	}

	return &synapse.ManagedIdentitySQLControlSettingsModel{
		ManagedIdentitySQLControlSettingsModelProperties: &synapse.ManagedIdentitySQLControlSettingsModelProperties{
			GrantSQLControlToManagedIdentity: &synapse.ManagedIdentitySQLControlSettingsModelPropertiesGrantSQLControlToManagedIdentity{
				DesiredState: desiredState,
			},
		},
	}
}

func expandEncryptionDetails(d *pluginsdk.ResourceData) *synapse.EncryptionDetails {
	if key, ok := d.GetOk("customer_managed_key_versionless_id"); ok {
		return &synapse.EncryptionDetails{
			Cmk: &synapse.CustomerManagedKeyDetails{
				Key: &synapse.WorkspaceKeyDetails{
					Name:        utils.String("cmk"),
					KeyVaultURL: utils.String(key.(string)),
				},
			},
		}
	}
	return nil
}

func flattenArmWorkspaceManagedIdentity(input *synapse.ManagedIdentity) []interface{} {
	if input == nil {
		return make([]interface{}, 0)
	}

	var principalId string
	if input.PrincipalID != nil {
		principalId = *input.PrincipalID
	}
	var tenantId string
	if input.TenantID != nil {
		tenantId = input.TenantID.String()
	}
	return []interface{}{
		map[string]interface{}{
			"type":         string(input.Type),
			"principal_id": principalId,
			"tenant_id":    tenantId,
		},
	}
}

func flattenArmWorkspaceDataLakeStorageAccountDetails(input *synapse.DataLakeStorageAccountDetails) string {
	if input != nil && input.AccountURL != nil && input.Filesystem != nil {
		return fmt.Sprintf("%s/%s", *input.AccountURL, *input.Filesystem)
	}
	return ""
}

func flattenArmWorkspaceAadAdmin(input *synapse.AadAdminProperties) []interface{} {
	if input == nil {
		return make([]interface{}, 0)
	}
	var tenantId, login, sid string
	if input.TenantID != nil {
		tenantId = *input.TenantID
	}
	if input.Login != nil {
		login = *input.Login
	}
	if input.Sid != nil {
		sid = *input.Sid
	}
	return []interface{}{
		map[string]interface{}{
			"tenant_id": tenantId,
			"login":     login,
			"object_id": sid,
		},
	}
}

func flattenWorkspaceRepositoryConfiguration(config *synapse.WorkspaceRepositoryConfiguration) (repoTypeResult string, result []interface{}) {
	if config == nil {
		return "", make([]interface{}, 0)
	}

	if repoType := config.Type; repoType != nil {
		repo := map[string]interface{}{}

		if *repoType == workspaceVSTSConfiguration {
			if config.ProjectName != nil {
				repo["project_name"] = *config.ProjectName
			}
			if config.TenantID != nil {
				repo["tenant_id"] = config.TenantID.String()
			}
		} else if *repoType == workspaceGitHubConfiguration {
			if config.HostName != nil {
				repo["git_url"] = *config.HostName
			}
		}

		if config.AccountName != nil {
			repo["account_name"] = *config.AccountName
		}
		if config.CollaborationBranch != nil {
			repo["branch_name"] = *config.CollaborationBranch
		}
		if config.RepositoryName != nil {
			repo["repository_name"] = *config.RepositoryName
		}
		if config.RootFolder != nil {
			repo["root_folder"] = *config.RootFolder
		}

		return *repoType, []interface{}{repo}
	}

	return "", make([]interface{}, 0)
}

func flattenIdentityControlSQLSettings(settings synapse.ManagedIdentitySQLControlSettingsModel) bool {
	if prop := settings.ManagedIdentitySQLControlSettingsModelProperties; prop != nil {
		if sqlControl := prop.GrantSQLControlToManagedIdentity; sqlControl != nil {
			if sqlControl.DesiredState == synapse.DesiredStateEnabled {
				return true
			}
		}
	}

	return false
}

func flattenEncryptionDetails(encryption *synapse.EncryptionDetails) *string {
	if cmk := encryption.Cmk; cmk != nil {
		if key := cmk.Key; key != nil {
			return key.KeyVaultURL
		}
	}
	return nil
}

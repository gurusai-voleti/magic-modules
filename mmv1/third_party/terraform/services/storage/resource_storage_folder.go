package storage

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-provider-google-beta/google-beta/tpgresource"
	transport_tpg "github.com/hashicorp/terraform-provider-google-beta/google-beta/transport"
	"github.com/hashicorp/terraform-provider-google-beta/google-beta/verify"

	"google.golang.org/api/storage/v1"
)

func ResourceStorageFolder() *schema.Resource {
	return &schema.Resource{
		Create: resourceStorageFolderCreate,
		Read:   resourceStorageFolderRead,
		Update: resourceStorageFolderUpdate,
		Delete: resourceStorageFolderDelete,

		Importer: &schema.ResourceImporter{
			State: resourceStorageFolderImport,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(20 * time.Minute),
			Update: schema.DefaultTimeout(20 * time.Minute),
			Delete: schema.DefaultTimeout(20 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"bucket": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: `The name of the bucket that contains the folder.`,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: verify.ValidateRegexp(`/$`),
				Description: `The name of the folder expressed as a path. Must include
trailing '/'. For example, 'example_dir/example_dir2/'.`,
			},
			"recursive": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: `Set to true if parent folder and child folders need to be created`,
			},
			"create_time": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: `The timestamp at which this folder was created.`,
			},
			"metageneration": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: `The metadata generation of the folder.`,
			},
			"update_time": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: `The timestamp at which this folder was most recently updated.`,
			},

			"self_link": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
		UseJSONNumber: true,
	}
}

func resourceStorageFolderCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*transport_tpg.Config)
	userAgent, err := tpgresource.GenerateUserAgentString(d, config.UserAgent)
	if err != nil {
		return err
	}

	obj := make(map[string]interface{})

	nameProp, err := expandStorageFolderName(d.Get("name"), d, config)
	if err != nil {
		return err
	} else if v, ok := d.GetOkExists("name"); !tpgresource.IsEmptyValue(reflect.ValueOf(nameProp)) && (ok || !reflect.DeepEqual(v, nameProp)) {
		obj["name"] = nameProp
	}

	url, err := tpgresource.ReplaceVars(d, config, "{{StorageBasePath}}b/{{bucket}}/folders?recursive={{recursive}}")
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Creating new Folder: %#v", obj)
	billingProject := ""

	// err == nil indicates that the billing_project value was found
	if bp, err := tpgresource.GetBillingProject(d, config); err == nil {
		billingProject = bp
	}

	headers := make(http.Header)
	res, err := transport_tpg.SendRequest(transport_tpg.SendRequestOptions{
		Config:    config,
		Method:    "POST",
		Project:   billingProject,
		RawURL:    url,
		UserAgent: userAgent,
		Body:      obj,
		Timeout:   d.Timeout(schema.TimeoutCreate),
		Headers:   headers,
	})
	if err != nil {
		return fmt.Errorf("Error creating Folder: %s", err)
	}

	// Store the ID now
	id, err := tpgresource.ReplaceVars(d, config, "{{bucket}}/{{name}}")
	if err != nil {
		return fmt.Errorf("Error constructing id: %s", err)
	}
	d.SetId(id)

	log.Printf("[DEBUG] Finished creating Folder %q: %#v", d.Id(), res)

	return resourceStorageFolderRead(d, meta)
}

func resourceStorageFolderRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*transport_tpg.Config)
	userAgent, err := tpgresource.GenerateUserAgentString(d, config.UserAgent)
	if err != nil {
		return err
	}

	url, err := tpgresource.ReplaceVars(d, config, "{{StorageBasePath}}b/{{bucket}}/folders/{{%name}}")
	if err != nil {
		return err
	}

	billingProject := ""

	// err == nil indicates that the billing_project value was found
	if bp, err := tpgresource.GetBillingProject(d, config); err == nil {
		billingProject = bp
	}

	headers := make(http.Header)
	res, err := transport_tpg.SendRequest(transport_tpg.SendRequestOptions{
		Config:    config,
		Method:    "GET",
		Project:   billingProject,
		RawURL:    url,
		UserAgent: userAgent,
		Headers:   headers,
	})
	if err != nil {
		return transport_tpg.HandleNotFoundError(err, d, fmt.Sprintf("StorageFolder %q", d.Id()))
	}

	if err := d.Set("create_time", flattenStorageFolderCreateTime(res["createTime"], d, config)); err != nil {
		return fmt.Errorf("Error reading Folder: %s", err)
	}
	if err := d.Set("update_time", flattenStorageFolderUpdateTime(res["updateTime"], d, config)); err != nil {
		return fmt.Errorf("Error reading Folder: %s", err)
	}
	if err := d.Set("metageneration", flattenStorageFolderMetageneration(res["metageneration"], d, config)); err != nil {
		return fmt.Errorf("Error reading Folder: %s", err)
	}
	if err := d.Set("bucket", flattenStorageFolderBucket(res["bucket"], d, config)); err != nil {
		return fmt.Errorf("Error reading Folder: %s", err)
	}
	if err := d.Set("name", flattenStorageFolderName(res["name"], d, config)); err != nil {
		return fmt.Errorf("Error reading Folder: %s", err)
	}
	if err := d.Set("recursive", flattenStorageFolderRecursive(res["recursive"], d, config)); err != nil {
		return fmt.Errorf("Error reading Folder: %s", err)
	}
	if err := d.Set("self_link", tpgresource.ConvertSelfLinkToV1(res["selfLink"].(string))); err != nil {
		return fmt.Errorf("Error reading Folder: %s", err)
	}

	return nil
}

func resourceStorageFolderUpdate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*transport_tpg.Config)
	userAgent, err := tpgresource.GenerateUserAgentString(d, config.UserAgent)
	if err != nil {
		return err
	}
	oldName, newName := d.GetChange("name")

	bucket := d.Get("bucket").(string)

	objectsService := storage.NewFoldersService(config.NewStorageClientWithTimeoutOverride(userAgent, d.Timeout(schema.TimeoutUpdate)))

	rename := objectsService.Rename(bucket, oldName.(string), newName.(string))
	_, err = rename.Do()

	if err != nil {
		return fmt.Errorf("Error updating folder %s: %s", oldName, err)
	}

	return resourceStorageFolderRead(d, meta)
}

func resourceStorageFolderDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*transport_tpg.Config)
	userAgent, err := tpgresource.GenerateUserAgentString(d, config.UserAgent)
	if err != nil {
		return err
	}

	billingProject := ""

	url, err := tpgresource.ReplaceVars(d, config, "{{StorageBasePath}}b/{{bucket}}/folders/{{%name}}")
	if err != nil {
		return err
	}

	var obj map[string]interface{}

	// err == nil indicates that the billing_project value was found
	if bp, err := tpgresource.GetBillingProject(d, config); err == nil {
		billingProject = bp
	}

	headers := make(http.Header)

	log.Printf("[DEBUG] Deleting Folder %q", d.Id())
	res, err := transport_tpg.SendRequest(transport_tpg.SendRequestOptions{
		Config:    config,
		Method:    "DELETE",
		Project:   billingProject,
		RawURL:    url,
		UserAgent: userAgent,
		Body:      obj,
		Timeout:   d.Timeout(schema.TimeoutDelete),
		Headers:   headers,
	})
	if err != nil {
		return transport_tpg.HandleNotFoundError(err, d, "Folder")
	}

	log.Printf("[DEBUG] Finished deleting Folder %q: %#v", d.Id(), res)
	return nil
}

func resourceStorageFolderImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	config := meta.(*transport_tpg.Config)
	if err := tpgresource.ParseImportId([]string{
		"^b/(?P<bucket>[^/]+)/folders/(?P<name>[^/]+)$",
		"^(?P<bucket>[^/]+)/(?P<name>[^/]+)$",
	}, d, config); err != nil {
		return nil, err
	}

	// Replace import id for the resource id
	id, err := tpgresource.ReplaceVars(d, config, "{{bucket}}/{{name}}")
	if err != nil {
		return nil, fmt.Errorf("Error constructing id: %s", err)
	}
	d.SetId(id)

	return []*schema.ResourceData{d}, nil
}

func flattenStorageFolderCreateTime(v interface{}, d *schema.ResourceData, config *transport_tpg.Config) interface{} {
	return v
}

func flattenStorageFolderUpdateTime(v interface{}, d *schema.ResourceData, config *transport_tpg.Config) interface{} {
	return v
}

func flattenStorageFolderMetageneration(v interface{}, d *schema.ResourceData, config *transport_tpg.Config) interface{} {
	return v
}

func flattenStorageFolderBucket(v interface{}, d *schema.ResourceData, config *transport_tpg.Config) interface{} {
	return v
}

func flattenStorageFolderName(v interface{}, d *schema.ResourceData, config *transport_tpg.Config) interface{} {
	return v
}

func flattenStorageFolderRecursive(v interface{}, d *schema.ResourceData, config *transport_tpg.Config) interface{} {
	return v
}

func expandStorageFolderBucket(v interface{}, d tpgresource.TerraformResourceData, config *transport_tpg.Config) (interface{}, error) {
	return v, nil
}

func expandStorageFolderName(v interface{}, d tpgresource.TerraformResourceData, config *transport_tpg.Config) (interface{}, error) {
	return v, nil
}

func expandStorageFolderRecursive(v interface{}, d tpgresource.TerraformResourceData, config *transport_tpg.Config) (interface{}, error) {
	return v, nil
}

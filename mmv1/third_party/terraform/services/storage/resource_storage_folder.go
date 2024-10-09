package storage

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"runtime"
	"strings"
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
				Default:     false,
				Description: `Set to true if parent folder and child folders need to be created`,
			},
			"force_destroy": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: `Set to true if non-empty folder needs to be force destroyed`,
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
	}
	path, _ := d.Get("name").(string)
	bucket, _ := d.Get("bucket").(string)

	var deleteObjectError error
	for deleteObjectError == nil {
		res, err := config.NewStorageClient(userAgent).Objects.List(bucket).Prefix(path).Do()
		if err != nil {
			log.Printf("Error listing contents of bucket %s and folder %s: %v", bucket, path, err)
			// If we can't list the contents, try deleting the bucket anyway in case it's empty

			break
		}

		if len(res.Items) == 0 {
			break // 0 items, bucket empty
		}

		if !d.Get("force_destroy").(bool) {
			deleteErr := fmt.Errorf("Error trying to delete folder %s containing objects without `force_destroy` set to true", path)
			log.Printf("Error! %s : %s\n\n", path, deleteErr)
			return deleteErr
		}

		wp := workerpool.New(runtime.NumCPU() - 1)

		for _, object := range res.Items {
			log.Printf("[DEBUG] Found %s", object.Name)
			object := object

			wp.Submit(func() {
				log.Printf("[TRACE] Attempting to delete %s", object.Name)
				if err := config.NewStorageClient(userAgent).Objects.Delete(bucket, object.Name).Do(); err != nil {
					deleteObjectError = err
					log.Printf("[ERR] Failed to delete storage object %s: %s", object.Name, err)
				} else {
					log.Printf("[TRACE] Successfully deleted %s", object.Name)
				}
			})
		}

		// Wait for everything to finish.
		wp.StopWait()

	}

	subpaths := strings.Split(path, "/")
	folder, _ := config.NewStorageClient(userAgent).Folders.List(bucket).Prefix(subpaths[0] + "/").Do()
	if folder.Items == nil && len(folder.Items) == 0 {
		return nil
	}
	folderItems := folder.Items
	for j := len(folderItems) - 1; j >= 0; j-- {
		err = config.NewStorageClient(userAgent).Folders.Delete(bucket, folderItems[j].Name).Do()
	}
	if err != nil {
		log.Printf("Unable to delete folder listing contents of bucket %s and folder %s: %v", bucket, path, err)
	}

	log.Printf("[DEBUG] Finished deleting Folder %q: %#v", d.Id(), path)
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

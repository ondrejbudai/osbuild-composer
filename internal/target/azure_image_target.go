package target

type AzureImageTargetOptions struct {
	Filename       string `json:"filename"`
	TenantID       string `json:"tenant_id"`
	Location       string `json:"location"`
	SubscriptionID string `json:"subscription_id"`
	ResourceGroup  string `json:"resource_group"`
}

func (AzureImageTargetOptions) isTargetOptions() {}

// NewAzureImageTarget creates org.osbuild.azure.image target
//
// This target uploads and registers an Azure Image. The image can be then
// immediately used to spin up a virtual machine.
//
// The target uses Azure OAuth credentials. In most cases you want to create
// a service principal for this purpose, see:
// https://docs.microsoft.com/en-us/azure/active-directory/develop/app-objects-and-service-principals
// The credentials are not passed in the target options, instead they are
// defined in the worker. If the worker doesn't have Azure credentials
// and gets a job with this target, the job will fail.
//
// The Tenant ID for the authorization process is specified in the target
// options. This means that this target can be used for multi-tenant
// applications.
//
// If you need to just upload a PageBlob into Azure Storage, see the
// org.osbuild.azure target.
func NewAzureImageTarget(options *AzureImageTargetOptions) *Target {
	return newTarget("org.osbuild.azure.image", options)
}

type AzureImageTargetResultOptions struct {
	ImageName string `json:"image_name"`
}

func (AzureImageTargetResultOptions) isTargetResultOptions() {}

func NewAzureImageTargetResult(options *AzureImageTargetResultOptions) *TargetResult {
	return newTargetResult("org.osbuild.azure.image", options)
}

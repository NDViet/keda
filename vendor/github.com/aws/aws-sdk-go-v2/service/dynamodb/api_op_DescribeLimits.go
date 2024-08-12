// Code generated by smithy-go-codegen DO NOT EDIT.

package dynamodb

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	internalEndpointDiscovery "github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Returns the current provisioned-capacity quotas for your Amazon Web Services
// account in a Region, both for the Region as a whole and for any one DynamoDB
// table that you create there.
//
// When you establish an Amazon Web Services account, the account has initial
// quotas on the maximum read capacity units and write capacity units that you can
// provision across all of your DynamoDB tables in a given Region. Also, there are
// per-table quotas that apply when you create a table there. For more information,
// see [Service, Account, and Table Quotas]page in the Amazon DynamoDB Developer Guide.
//
// Although you can increase these quotas by filing a case at [Amazon Web Services Support Center], obtaining the
// increase is not instantaneous. The DescribeLimits action lets you write code to
// compare the capacity you are currently using to those quotas imposed by your
// account so that you have enough time to apply for an increase before you hit a
// quota.
//
// For example, you could use one of the Amazon Web Services SDKs to do the
// following:
//
//   - Call DescribeLimits for a particular Region to obtain your current account
//     quotas on provisioned capacity there.
//
//   - Create a variable to hold the aggregate read capacity units provisioned for
//     all your tables in that Region, and one to hold the aggregate write capacity
//     units. Zero them both.
//
//   - Call ListTables to obtain a list of all your DynamoDB tables.
//
//   - For each table name listed by ListTables , do the following:
//
//   - Call DescribeTable with the table name.
//
//   - Use the data returned by DescribeTable to add the read capacity units and
//     write capacity units provisioned for the table itself to your variables.
//
//   - If the table has one or more global secondary indexes (GSIs), loop over
//     these GSIs and add their provisioned capacity values to your variables as well.
//
//   - Report the account quotas for that Region returned by DescribeLimits , along
//     with the total current provisioned capacity levels you have calculated.
//
// This will let you see whether you are getting close to your account-level
// quotas.
//
// The per-table quotas apply only when you are creating a new table. They
// restrict the sum of the provisioned capacity of the new table itself and all its
// global secondary indexes.
//
// For existing tables and their GSIs, DynamoDB doesn't let you increase
// provisioned capacity extremely rapidly, but the only quota that applies is that
// the aggregate provisioned capacity over all your tables and GSIs cannot exceed
// either of the per-account quotas.
//
// DescribeLimits should only be called periodically. You can expect throttling
// errors if you call it more than once in a minute.
//
// The DescribeLimits Request element has no content.
//
// [Service, Account, and Table Quotas]: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Limits.html
// [Amazon Web Services Support Center]: https://console.aws.amazon.com/support/home#/
func (c *Client) DescribeLimits(ctx context.Context, params *DescribeLimitsInput, optFns ...func(*Options)) (*DescribeLimitsOutput, error) {
	if params == nil {
		params = &DescribeLimitsInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "DescribeLimits", params, optFns, c.addOperationDescribeLimitsMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*DescribeLimitsOutput)
	out.ResultMetadata = metadata
	return out, nil
}

// Represents the input of a DescribeLimits operation. Has no content.
type DescribeLimitsInput struct {
	noSmithyDocumentSerde
}

// Represents the output of a DescribeLimits operation.
type DescribeLimitsOutput struct {

	// The maximum total read capacity units that your account allows you to provision
	// across all of your tables in this Region.
	AccountMaxReadCapacityUnits *int64

	// The maximum total write capacity units that your account allows you to
	// provision across all of your tables in this Region.
	AccountMaxWriteCapacityUnits *int64

	// The maximum read capacity units that your account allows you to provision for a
	// new table that you are creating in this Region, including the read capacity
	// units provisioned for its global secondary indexes (GSIs).
	TableMaxReadCapacityUnits *int64

	// The maximum write capacity units that your account allows you to provision for
	// a new table that you are creating in this Region, including the write capacity
	// units provisioned for its global secondary indexes (GSIs).
	TableMaxWriteCapacityUnits *int64

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationDescribeLimitsMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsjson10_serializeOpDescribeLimits{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsjson10_deserializeOpDescribeLimits{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "DescribeLimits"); err != nil {
		return fmt.Errorf("add protocol finalizers: %v", err)
	}

	if err = addlegacyEndpointContextSetter(stack, options); err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = addClientRequestID(stack); err != nil {
		return err
	}
	if err = addComputeContentLength(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = addComputePayloadSHA256(stack); err != nil {
		return err
	}
	if err = addRetry(stack, options); err != nil {
		return err
	}
	if err = addRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = addRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addClientUserAgent(stack, options); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addOpDescribeLimitsDiscoverEndpointMiddleware(stack, options, c); err != nil {
		return err
	}
	if err = addSetLegacyContextSigningOptionsMiddleware(stack); err != nil {
		return err
	}
	if err = addTimeOffsetBuild(stack, c); err != nil {
		return err
	}
	if err = addUserAgentRetryMode(stack, options); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opDescribeLimits(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addRecursionDetection(stack); err != nil {
		return err
	}
	if err = addRequestIDRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = addValidateResponseChecksum(stack, options); err != nil {
		return err
	}
	if err = addAcceptEncodingGzip(stack, options); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	if err = addDisableHTTPSMiddleware(stack, options); err != nil {
		return err
	}
	return nil
}

func addOpDescribeLimitsDiscoverEndpointMiddleware(stack *middleware.Stack, o Options, c *Client) error {
	return stack.Finalize.Insert(&internalEndpointDiscovery.DiscoverEndpoint{
		Options: []func(*internalEndpointDiscovery.DiscoverEndpointOptions){
			func(opt *internalEndpointDiscovery.DiscoverEndpointOptions) {
				opt.DisableHTTPS = o.EndpointOptions.DisableHTTPS
				opt.Logger = o.Logger
			},
		},
		DiscoverOperation:            c.fetchOpDescribeLimitsDiscoverEndpoint,
		EndpointDiscoveryEnableState: o.EndpointDiscovery.EnableEndpointDiscovery,
		EndpointDiscoveryRequired:    false,
		Region:                       o.Region,
	}, "ResolveEndpointV2", middleware.After)
}

func (c *Client) fetchOpDescribeLimitsDiscoverEndpoint(ctx context.Context, region string, optFns ...func(*internalEndpointDiscovery.DiscoverEndpointOptions)) (internalEndpointDiscovery.WeightedAddress, error) {
	input := getOperationInput(ctx)
	in, ok := input.(*DescribeLimitsInput)
	if !ok {
		return internalEndpointDiscovery.WeightedAddress{}, fmt.Errorf("unknown input type %T", input)
	}
	_ = in

	identifierMap := make(map[string]string, 0)
	identifierMap["sdk#Region"] = region

	key := fmt.Sprintf("DynamoDB.%v", identifierMap)

	if v, ok := c.endpointCache.Get(key); ok {
		return v, nil
	}

	discoveryOperationInput := &DescribeEndpointsInput{}

	opt := internalEndpointDiscovery.DiscoverEndpointOptions{}
	for _, fn := range optFns {
		fn(&opt)
	}

	go c.handleEndpointDiscoveryFromService(ctx, discoveryOperationInput, region, key, opt)
	return internalEndpointDiscovery.WeightedAddress{}, nil
}

func newServiceMetadataMiddleware_opDescribeLimits(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "DescribeLimits",
	}
}

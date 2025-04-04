/*
Copyright 2021 The KEDA Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scaling

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/expr-lang/expr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	v2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/metrics/pkg/apis/external_metrics"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/kedacore/keda/v2/pkg/mock/mock_client"
	mock_scalers "github.com/kedacore/keda/v2/pkg/mock/mock_scaler"
	"github.com/kedacore/keda/v2/pkg/mock/mock_scaling/mock_executor"
	"github.com/kedacore/keda/v2/pkg/scalers"
	"github.com/kedacore/keda/v2/pkg/scalers/authentication"
	"github.com/kedacore/keda/v2/pkg/scalers/scalersconfig"
	"github.com/kedacore/keda/v2/pkg/scaling/cache"
	"github.com/kedacore/keda/v2/pkg/scaling/cache/metricscache"
)

const testNamespaceGlobal = "testNamespace"
const compositeMetricNameGlobal = "composite-metric"
const testNameGlobal = "testName"

func TestGetScaledObjectMetrics_DirectCall(t *testing.T) {
	scaledObjectName := testNameGlobal
	scaledObjectNamespace := testNamespaceGlobal
	metricName := "test-metric-name"
	longPollingInterval := int32(300)

	ctrl := gomock.NewController(t)
	recorder := record.NewFakeRecorder(1)
	mockClient := mock_client.NewMockClient(ctrl)
	mockExecutor := mock_executor.NewMockScaleExecutor(ctrl)

	metricsSpecs := []v2.MetricSpec{createMetricSpec(10, metricName)}
	metricValue := scalers.GenerateMetricInMili(metricName, float64(10))

	metricsRecord := map[string]metricscache.MetricsRecord{}
	metricsRecord[metricName] = metricscache.MetricsRecord{
		IsActive:    true,
		Metric:      []external_metrics.ExternalMetricValue{metricValue},
		ScalerError: nil,
	}

	scaler := mock_scalers.NewMockScaler(ctrl)
	// we are going to query metrics directly
	scalerConfig := scalersconfig.ScalerConfig{TriggerUseCachedMetrics: false}
	factory := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		return scaler, &scalerConfig, nil
	}

	scaledObject := kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scaledObjectName,
			Namespace: scaledObjectNamespace,
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: "test",
			},
			PollingInterval: &longPollingInterval,
		},
		Status: kedav1alpha1.ScaledObjectStatus{
			ScaleTargetGVKR: &kedav1alpha1.GroupVersionKindResource{
				Group: "apps",
				Kind:  "Deployment",
			},
		},
	}

	scalerCache := cache.ScalersCache{
		ScaledObject: &scaledObject,
		Scalers: []cache.ScalerBuilder{{
			Scaler:       scaler,
			ScalerConfig: scalerConfig,
			Factory:      factory,
		}},
		Recorder: recorder,
	}

	caches := map[string]*cache.ScalersCache{}
	caches[scaledObject.GenerateIdentifier()] = &scalerCache

	sh := scaleHandler{
		client:                   mockClient,
		scaleLoopContexts:        &sync.Map{},
		scaleExecutor:            mockExecutor,
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             caches,
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
	}

	mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	scaler.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs)
	scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{metricValue}, true, nil)
	mockExecutor.EXPECT().RequestScale(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	sh.checkScalers(context.TODO(), &scaledObject, &sync.RWMutex{})

	expectNoStatusPatch(ctrl)
	scaler.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs)
	// hitting directly GetMetricsAndActivity()
	scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{metricValue}, true, nil)
	metrics, err := sh.GetScaledObjectMetrics(context.TODO(), scaledObjectName, scaledObjectNamespace, metricName)
	assert.NotNil(t, metrics)
	assert.Nil(t, err)

	scaler.EXPECT().Close(gomock.Any())
	scalerCache.Close(context.Background())
}

func TestGetScaledObjectMetrics_FromCache(t *testing.T) {
	scaledObjectName := "testName2"
	scaledObjectNamespace := "testNamespace2"
	metricName := "test-metric-name2"
	longPollingInterval := int32(300)

	ctrl := gomock.NewController(t)
	recorder := record.NewFakeRecorder(1)
	mockClient := mock_client.NewMockClient(ctrl)
	mockExecutor := mock_executor.NewMockScaleExecutor(ctrl)

	metricsSpecs := []v2.MetricSpec{createMetricSpec(10, metricName)}
	metricValue := scalers.GenerateMetricInMili(metricName, float64(10))

	metricsRecord := map[string]metricscache.MetricsRecord{}
	metricsRecord[metricName] = metricscache.MetricsRecord{
		IsActive:    true,
		Metric:      []external_metrics.ExternalMetricValue{metricValue},
		ScalerError: nil,
	}

	scaler := mock_scalers.NewMockScaler(ctrl)
	// we are going to use cache for metrics values
	scalerConfig := scalersconfig.ScalerConfig{TriggerUseCachedMetrics: true}
	factory := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		return scaler, &scalerConfig, nil
	}

	scaledObject := kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scaledObjectName,
			Namespace: scaledObjectNamespace,
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: "test",
			},
			PollingInterval: &longPollingInterval,
		},
		Status: kedav1alpha1.ScaledObjectStatus{
			ScaleTargetGVKR: &kedav1alpha1.GroupVersionKindResource{
				Group: "apps",
				Kind:  "Deployment",
			},
		},
	}

	scalerCache := cache.ScalersCache{
		ScaledObject: &scaledObject,
		Scalers: []cache.ScalerBuilder{{
			Scaler:       scaler,
			ScalerConfig: scalerConfig,
			Factory:      factory,
		}},
		Recorder: recorder,
	}

	caches := map[string]*cache.ScalersCache{}
	caches[scaledObject.GenerateIdentifier()] = &scalerCache

	sh := scaleHandler{
		client:                   mockClient,
		scaleLoopContexts:        &sync.Map{},
		scaleExecutor:            mockExecutor,
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             caches,
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
	}

	mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	scaler.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs)
	scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{metricValue}, true, nil)
	mockExecutor.EXPECT().RequestScale(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	sh.checkScalers(context.TODO(), &scaledObject, &sync.RWMutex{})

	expectNoStatusPatch(ctrl)
	scaler.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs)
	// hitting cache here instead of calling GetMetricsAndActivity()
	metrics, err := sh.GetScaledObjectMetrics(context.TODO(), scaledObjectName, scaledObjectNamespace, metricName)
	assert.NotNil(t, metrics)
	assert.Nil(t, err)

	scaler.EXPECT().Close(gomock.Any())
	scalerCache.Close(context.Background())
}

// TestGetScaledObjectMetrics_InParallel executes
// a request to multiple scalers with a delay.
// The sum off all the scalers is more than the timeout
// but all of them in parallel are recovered in time
func TestGetScaledObjectMetrics_InParallel(t *testing.T) {
	scaledObjectName := testNameGlobal
	scaledObjectNamespace := testNamespaceGlobal
	metricNames := []string{
		"test-metric-name-1",
		"test-metric-name-2",
		"test-metric-name-3",
		"test-metric-name-4",
		"test-metric-name-5",
		"test-metric-name-6",
		"test-metric-name-7",
		"test-metric-name-8",
		"test-metric-name-9",
		"test-metric-name-10",
	}
	metricsName := strings.Join(metricNames, ";")
	longPollingInterval := int32(300)

	ctrl := gomock.NewController(t)
	recorder := record.NewFakeRecorder(1)
	mockClient := mock_client.NewMockClient(ctrl)
	mockExecutor := mock_executor.NewMockScaleExecutor(ctrl)

	scalerCollection := []*mock_scalers.MockScaler{}

	for i := 0; i < len(metricNames); i++ {
		scalerCollection = append(scalerCollection, mock_scalers.NewMockScaler(ctrl))
	}

	metricsSpecFn := func(index int) []v2.MetricSpec {
		return []v2.MetricSpec{createMetricSpec(10, metricNames[index])}
	}
	metricsValueFn := func(index int) []external_metrics.ExternalMetricValue {
		time.Sleep(200 * time.Millisecond)
		return []external_metrics.ExternalMetricValue{scalers.GenerateMetricInMili(metricNames[index], float64(10))}
	}
	scalerConfigFn := func(index int) *scalersconfig.ScalerConfig {
		return &scalersconfig.ScalerConfig{
			TriggerUseCachedMetrics: false,
			TriggerIndex:            index,
		}
	}

	scalerFactoryFn := func(index int) func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		return func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
			return scalerCollection[index], scalerConfigFn(index), nil
		}
	}

	scaledObject := kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scaledObjectName,
			Namespace: scaledObjectNamespace,
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: "test",
			},
			PollingInterval: &longPollingInterval,
			Advanced: &kedav1alpha1.AdvancedConfig{
				ScalingModifiers: kedav1alpha1.ScalingModifiers{
					Target: "1",
				},
			},
		},
		Status: kedav1alpha1.ScaledObjectStatus{
			ScaleTargetGVKR: &kedav1alpha1.GroupVersionKindResource{
				Group: "apps",
				Kind:  "Deployment",
			},
			ExternalMetricNames: metricNames,
		},
	}

	scalerCache := cache.ScalersCache{
		ScaledObject: &scaledObject,
		Scalers:      []cache.ScalerBuilder{},
		Recorder:     recorder,
	}
	for i := 0; i < len(metricNames); i++ {
		scalerCache.Scalers = append(scalerCache.Scalers, cache.ScalerBuilder{
			Scaler:       scalerCollection[i],
			ScalerConfig: *scalerConfigFn(i),
			Factory:      scalerFactoryFn(i),
		})
	}

	caches := map[string]*cache.ScalersCache{}
	caches[scaledObject.GenerateIdentifier()] = &scalerCache

	sh := scaleHandler{
		client:                   mockClient,
		scaleLoopContexts:        &sync.Map{},
		scaleExecutor:            mockExecutor,
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             caches,
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
	}

	mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	for i := 0; i < len(metricNames); i++ {
		scalerCollection[i].EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecFn(i))
		scalerCollection[i].EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, metricName string) ([]external_metrics.ExternalMetricValue, bool, error) {
			return metricsValueFn(i), true, nil
		})
	}
	mockExecutor.EXPECT().RequestScale(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	assert.Eventually(t, func() bool {
		sh.checkScalers(context.TODO(), &scaledObject, &sync.RWMutex{})
		return true
	}, 1*time.Second, 400*time.Millisecond, "timeout exceeded: scalers not processed in parallel during `checkScalers`")

	expectNoStatusPatch(ctrl)

	for i := 0; i < len(metricNames); i++ {
		scalerCollection[i].EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecFn(i))
		scalerCollection[i].EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, metricName string) ([]external_metrics.ExternalMetricValue, bool, error) {
			return metricsValueFn(i), true, nil
		})
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		metrics, err := sh.GetScaledObjectMetrics(context.TODO(), scaledObjectName, scaledObjectNamespace, metricsName)
		assert.NotNil(c, metrics)
		assert.Nil(c, err)
	}, 1*time.Second, 400*time.Millisecond, "timeout exceeded: scalers not processed in parallel during `GetScaledObjectMetrics`")

	for i := 0; i < len(metricNames); i++ {
		scalerCollection[i].EXPECT().Close(gomock.Any())
	}
	scalerCache.Close(context.Background())
}

func TestCheckScaledObjectScalersWithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(ctrl)
	mockExecutor := mock_executor.NewMockScaleExecutor(ctrl)
	recorder := record.NewFakeRecorder(1)

	metricsSpecs := []v2.MetricSpec{createMetricSpec(1, "metric-name")}

	scaler := mock_scalers.NewMockScaler(ctrl)
	scaler.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs)
	scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{}, false, errors.New("some error"))
	scaler.EXPECT().Close(gomock.Any())

	factory := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		scaler := mock_scalers.NewMockScaler(ctrl)
		scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{}, false, errors.New("some error"))
		scaler.EXPECT().Close(gomock.Any())
		return scaler, &scalersconfig.ScalerConfig{}, nil
	}

	scaledObject := kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: "test",
			},
		},
	}

	scalerCache := cache.ScalersCache{
		Scalers: []cache.ScalerBuilder{{
			Scaler:  scaler,
			Factory: factory,
		}},
		Recorder: recorder,
	}

	caches := map[string]*cache.ScalersCache{}
	caches[scaledObject.GenerateIdentifier()] = &scalerCache

	sh := scaleHandler{
		client:                   mockClient,
		scaleLoopContexts:        &sync.Map{},
		scaleExecutor:            mockExecutor,
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             caches,
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
	}

	isActive, isError, _, activeTriggers, _ := sh.getScaledObjectState(context.TODO(), &scaledObject)
	scalerCache.Close(context.Background())

	assert.Equal(t, false, isActive)
	assert.Equal(t, true, isError)
	assert.Empty(t, activeTriggers)
}

func TestCheckScaledObjectScalersWithTriggerAuthError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(ctrl)
	mockExecutor := mock_executor.NewMockScaleExecutor(ctrl)
	recorder := record.NewFakeRecorder(1)

	scaler := mock_scalers.NewMockScaler(ctrl)
	scaler.EXPECT().Close(gomock.Any())

	factory := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		scaler := mock_scalers.NewMockScaler(ctrl)
		scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{}, false, errors.New("some error"))
		scaler.EXPECT().Close(gomock.Any())
		return scaler, &scalersconfig.ScalerConfig{}, nil
	}

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "container",
						},
					},
				},
			},
		},
	}

	scaledObject := kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scaledobject-test",
			Namespace: "test",
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: deployment.Name,
			},
			Triggers: []kedav1alpha1.ScaleTriggers{
				{
					Name: triggerName1,
					Type: "fake_trig1",
					AuthenticationRef: &kedav1alpha1.AuthenticationRef{
						Name: "triggerauth-test",
					},
				},
			},
		},
		Status: kedav1alpha1.ScaledObjectStatus{
			ScaleTargetGVKR: &kedav1alpha1.GroupVersionKindResource{
				Group: "apps",
				Kind:  "Deployment",
			},
			ExternalMetricNames: []string{metricName1, metricName2},
		},
	}

	triggerAuth := kedav1alpha1.TriggerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "triggerauth-test",
			Namespace: "test",
		},
		Spec: kedav1alpha1.TriggerAuthenticationSpec{
			HashiCorpVault: &kedav1alpha1.HashiCorpVault{
				Address:        "invalid-vault-address",
				Authentication: "token",
				Credential: &kedav1alpha1.Credential{
					Token: "my-token",
				},
				Mount: "kubernetes",
				Role:  "my-role",
				Secrets: []kedav1alpha1.VaultSecret{
					{
						Parameter: "username",
						Key:       "username",
						Path:      "secret_v2/data/my-username-path",
					},
				},
			},
		},
	}

	scalerCache := cache.ScalersCache{
		Scalers: []cache.ScalerBuilder{{
			Scaler:  scaler,
			Factory: factory,
		}},
		Recorder: recorder,
	}

	mockClient.EXPECT().Get(gomock.Any(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, gomock.Any()).SetArg(2, deployment)
	mockClient.EXPECT().Get(gomock.Any(), types.NamespacedName{Name: triggerAuth.Name, Namespace: triggerAuth.Namespace}, gomock.Any()).SetArg(2, triggerAuth)

	sh := scaleHandler{
		client:                   mockClient,
		scaleLoopContexts:        &sync.Map{},
		scaleExecutor:            mockExecutor,
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             map[string]*cache.ScalersCache{},
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
		authClientSet: &authentication.AuthClientSet{
			SecretLister: nil,
		},
	}

	isActive, isError, _, activeTriggers, _ := sh.getScaledObjectState(context.TODO(), &scaledObject)
	scalerCache.Close(context.Background())

	assert.Equal(t, false, isActive)
	assert.Equal(t, true, isError)
	assert.Empty(t, activeTriggers)

	failureEvent := <-recorder.Events
	assert.Contains(t, failureEvent, "KEDAScalerFailed")
	assert.Contains(t, failureEvent, "unsupported protocol scheme")
}

func TestCheckScaledObjectFindFirstActiveNotIgnoreOthers(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mock_client.NewMockClient(ctrl)
	mockExecutor := mock_executor.NewMockScaleExecutor(ctrl)
	recorder := record.NewFakeRecorder(1)

	metricsSpecs := []v2.MetricSpec{createMetricSpec(1, "metric-name")}

	activeFactory := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		scaler := mock_scalers.NewMockScaler(ctrl)
		scaler.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs)
		scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{}, true, nil)
		scaler.EXPECT().Close(gomock.Any())
		return scaler, &scalersconfig.ScalerConfig{}, nil
	}
	activeScaler, _, err := activeFactory()
	assert.Nil(t, err)

	failingFactory := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		scaler := mock_scalers.NewMockScaler(ctrl)
		scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{}, false, errors.New("some error"))
		scaler.EXPECT().Close(gomock.Any())
		return scaler, &scalersconfig.ScalerConfig{}, nil
	}
	failingScaler := mock_scalers.NewMockScaler(ctrl)
	failingScaler.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs)
	failingScaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{}, false, errors.New("some error"))
	failingScaler.EXPECT().Close(gomock.Any())

	scaledObject := kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: "test",
			},
		},
	}

	scalers := []cache.ScalerBuilder{{
		Scaler:  activeScaler,
		Factory: activeFactory,
	}, {
		Scaler:  failingScaler,
		Factory: failingFactory,
	}}

	scalerCache := cache.ScalersCache{
		Scalers:  scalers,
		Recorder: recorder,
	}

	caches := map[string]*cache.ScalersCache{}
	caches[scaledObject.GenerateIdentifier()] = &scalerCache

	sh := scaleHandler{
		client:                   mockClient,
		scaleLoopContexts:        &sync.Map{},
		scaleExecutor:            mockExecutor,
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             caches,
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
	}

	isActive, isError, _, activeTriggers, _ := sh.getScaledObjectState(context.TODO(), &scaledObject)
	scalerCache.Close(context.Background())

	assert.Equal(t, true, isActive)
	assert.Equal(t, true, isError)
	assert.Equal(t, []string{"*mock_scalers.MockScaler"}, activeTriggers)
}

func TestIsScaledJobActive(t *testing.T) {
	metricName := "s0-queueLength"
	ctrl := gomock.NewController(t)
	recorder := record.NewFakeRecorder(1)
	// Keep the current behavior
	// Assme 1 trigger only
	scaledJobSingle := createScaledJob(1, 100, "") // testing default = max
	scalerCache := cache.ScalersCache{
		Scalers: []cache.ScalerBuilder{{
			Scaler: createScaler(ctrl, int64(20), int64(2), true, metricName),
			Factory: func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
				return createScaler(ctrl, int64(20), int64(2), true, metricName), &scalersconfig.ScalerConfig{}, nil
			},
		}},
		Recorder: recorder,
	}

	caches := map[string]*cache.ScalersCache{}
	caches[scaledJobSingle.GenerateIdentifier()] = &scalerCache

	sh := scaleHandler{
		scaleLoopContexts:        &sync.Map{},
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             caches,
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
	}
	// nosemgrep: context-todo
	isActive, isError, queueLength, maxValue := sh.isScaledJobActive(context.TODO(), scaledJobSingle)
	assert.Equal(t, true, isActive)
	assert.Equal(t, false, isError)
	assert.Equal(t, int64(20), queueLength)
	assert.Equal(t, int64(10), maxValue)
	scalerCache.Close(context.Background())

	// Test the valiation
	scalerTestDatam := []scalerTestData{
		newScalerTestData("s0-queueLength", 100, "max", 20, 1, true, 10, 2, true, 5, 3, true, 7, 4, false, true, false, 20, 20),
		newScalerTestData("queueLength", 100, "min", 20, 1, true, 10, 2, true, 5, 3, true, 7, 4, false, true, false, 5, 2),
		newScalerTestData("messageCount", 100, "avg", 20, 1, true, 10, 2, true, 5, 3, true, 7, 4, false, true, false, 12, 9),
		newScalerTestData("s3-messageCount", 100, "sum", 20, 1, true, 10, 2, true, 5, 3, true, 7, 4, false, true, false, 35, 27),
		newScalerTestData("s10-messageCount", 25, "sum", 20, 1, true, 10, 2, true, 5, 3, true, 7, 4, false, true, false, 35, 25),
	}

	for index, scalerTestData := range scalerTestDatam {
		scaledJob := createScaledJob(scalerTestData.MinReplicaCount, scalerTestData.MaxReplicaCount, scalerTestData.MultipleScalersCalculation)
		scalersToTest := []cache.ScalerBuilder{{
			Scaler: createScaler(ctrl, scalerTestData.Scaler1QueueLength, scalerTestData.Scaler1AverageValue, scalerTestData.Scaler1IsActive, scalerTestData.MetricName),
			Factory: func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
				return createScaler(ctrl, scalerTestData.Scaler1QueueLength, scalerTestData.Scaler1AverageValue, scalerTestData.Scaler1IsActive, scalerTestData.MetricName), &scalersconfig.ScalerConfig{}, nil
			},
		}, {
			Scaler: createScaler(ctrl, scalerTestData.Scaler2QueueLength, scalerTestData.Scaler2AverageValue, scalerTestData.Scaler2IsActive, scalerTestData.MetricName),
			Factory: func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
				return createScaler(ctrl, scalerTestData.Scaler2QueueLength, scalerTestData.Scaler2AverageValue, scalerTestData.Scaler2IsActive, scalerTestData.MetricName), &scalersconfig.ScalerConfig{}, nil
			},
		}, {
			Scaler: createScaler(ctrl, scalerTestData.Scaler3QueueLength, scalerTestData.Scaler3AverageValue, scalerTestData.Scaler3IsActive, scalerTestData.MetricName),
			Factory: func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
				return createScaler(ctrl, scalerTestData.Scaler3QueueLength, scalerTestData.Scaler3AverageValue, scalerTestData.Scaler3IsActive, scalerTestData.MetricName), &scalersconfig.ScalerConfig{}, nil
			},
		}, {
			Scaler: createScaler(ctrl, scalerTestData.Scaler4QueueLength, scalerTestData.Scaler4AverageValue, scalerTestData.Scaler4IsActive, scalerTestData.MetricName),
			Factory: func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
				return createScaler(ctrl, scalerTestData.Scaler4QueueLength, scalerTestData.Scaler4AverageValue, scalerTestData.Scaler4IsActive, scalerTestData.MetricName), &scalersconfig.ScalerConfig{}, nil
			},
		}}

		scalerCache = cache.ScalersCache{
			Scalers:  scalersToTest,
			Recorder: recorder,
		}

		caches = map[string]*cache.ScalersCache{}
		caches[scaledJobSingle.GenerateIdentifier()] = &scalerCache

		sh = scaleHandler{
			scaleLoopContexts:        &sync.Map{},
			globalHTTPTimeout:        time.Duration(1000),
			recorder:                 recorder,
			scalerCaches:             caches,
			scalerCachesLock:         &sync.RWMutex{},
			scaledObjectsMetricCache: metricscache.NewMetricsCache(),
		}
		fmt.Printf("index: %d", index)
		// nosemgrep: context-todo
		isActive, isError, queueLength, maxValue = sh.isScaledJobActive(context.TODO(), scaledJob)
		//	assert.Equal(t, 5, index)
		assert.Equal(t, scalerTestData.ResultIsActive, isActive)
		assert.Equal(t, scalerTestData.ResultIsError, isError)
		assert.Equal(t, scalerTestData.ResultQueueLength, queueLength)
		assert.Equal(t, scalerTestData.ResultMaxValue, maxValue)
		scalerCache.Close(context.Background())
	}
}

func TestIsScaledJobActiveIfQueueEmptyButMinReplicaCountGreaterZero(t *testing.T) {
	metricName := "s0-queueLength"
	ctrl := gomock.NewController(t)
	recorder := record.NewFakeRecorder(1)
	// Keep the current behavior
	// Assme 1 trigger only
	scaledJobSingle := createScaledJob(1, 100, "") // testing default = max
	scalerSingle := []cache.ScalerBuilder{{
		Scaler: createScaler(ctrl, int64(0), int64(1), true, metricName),
		Factory: func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
			return createScaler(ctrl, int64(0), int64(1), true, metricName), &scalersconfig.ScalerConfig{}, nil
		},
	}}

	scalerCache := cache.ScalersCache{
		Scalers:  scalerSingle,
		Recorder: recorder,
	}

	caches := map[string]*cache.ScalersCache{}
	caches[scaledJobSingle.GenerateIdentifier()] = &scalerCache

	sh := scaleHandler{
		scaleLoopContexts:        &sync.Map{},
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             caches,
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
	}

	// nosemgrep: context-todo
	isActive, isError, queueLength, maxValue := sh.isScaledJobActive(context.TODO(), scaledJobSingle)
	assert.Equal(t, true, isActive)
	assert.Equal(t, false, isError)
	assert.Equal(t, int64(0), queueLength)
	assert.Equal(t, int64(0), maxValue)
	scalerCache.Close(context.Background())
}

func newScalerTestData(
	metricName string,
	maxReplicaCount int,
	multipleScalersCalculation string,
	scaler1QueueLength, //nolint:golint,unparam
	scaler1AverageValue int, //nolint:golint,unparam
	scaler1IsActive bool, //nolint:golint,unparam
	scaler2QueueLength, //nolint:golint,unparam
	scaler2AverageValue int, //nolint:golint,unparam
	scaler2IsActive bool, //nolint:golint,unparam
	scaler3QueueLength, //nolint:golint,unparam
	scaler3AverageValue int, //nolint:golint,unparam
	scaler3IsActive bool, //nolint:golint,unparam
	scaler4QueueLength, //nolint:golint,unparam
	scaler4AverageValue int, //nolint:golint,unparam
	scaler4IsActive bool, //nolint:golint,unparam
	resultIsActive bool, //nolint:golint,unparam
	resultIsError bool, //nolint:golint,unparam
	resultQueueLength,
	resultMaxLength int) scalerTestData {
	return scalerTestData{
		MetricName:                 metricName,
		MaxReplicaCount:            int32(maxReplicaCount),
		MultipleScalersCalculation: multipleScalersCalculation,
		Scaler1QueueLength:         int64(scaler1QueueLength),
		Scaler1AverageValue:        int64(scaler1AverageValue),
		Scaler1IsActive:            scaler1IsActive,
		Scaler2QueueLength:         int64(scaler2QueueLength),
		Scaler2AverageValue:        int64(scaler2AverageValue),
		Scaler2IsActive:            scaler2IsActive,
		Scaler3QueueLength:         int64(scaler3QueueLength),
		Scaler3AverageValue:        int64(scaler3AverageValue),
		Scaler3IsActive:            scaler3IsActive,
		Scaler4QueueLength:         int64(scaler4QueueLength),
		Scaler4AverageValue:        int64(scaler4AverageValue),
		Scaler4IsActive:            scaler4IsActive,
		ResultIsActive:             resultIsActive,
		ResultIsError:              resultIsError,
		ResultQueueLength:          int64(resultQueueLength),
		ResultMaxValue:             int64(resultMaxLength),
	}
}

type scalerTestData struct {
	MetricName                 string
	MaxReplicaCount            int32
	MultipleScalersCalculation string
	Scaler1QueueLength         int64
	Scaler1AverageValue        int64
	Scaler1IsActive            bool
	Scaler2QueueLength         int64
	Scaler2AverageValue        int64
	Scaler2IsActive            bool
	Scaler3QueueLength         int64
	Scaler3AverageValue        int64
	Scaler3IsActive            bool
	Scaler4QueueLength         int64
	Scaler4AverageValue        int64
	Scaler4IsActive            bool
	ResultIsActive             bool
	ResultIsError              bool
	ResultQueueLength          int64
	ResultMaxValue             int64
	MinReplicaCount            int32
}

func createScaledJob(minReplicaCount int32, maxReplicaCount int32, multipleScalersCalculation string) *kedav1alpha1.ScaledJob {
	if multipleScalersCalculation != "" {
		return &kedav1alpha1.ScaledJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: kedav1alpha1.ScaledJobSpec{
				MinReplicaCount: &minReplicaCount,
				MaxReplicaCount: &maxReplicaCount,
				ScalingStrategy: kedav1alpha1.ScalingStrategy{
					MultipleScalersCalculation: multipleScalersCalculation,
				},
				JobTargetRef: &batchv1.JobSpec{
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "test",
						},
					},
				},
				EnvSourceContainerName: "test",
			},
		}
	}
	return &kedav1alpha1.ScaledJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: kedav1alpha1.ScaledJobSpec{
			MinReplicaCount: &minReplicaCount,
			MaxReplicaCount: &maxReplicaCount,
			JobTargetRef: &batchv1.JobSpec{
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
				},
			},
			EnvSourceContainerName: "test",
		},
	}
}

func createScaler(ctrl *gomock.Controller, queueLength int64, averageValue int64, isActive bool, metricName string) *mock_scalers.MockScaler {
	scaler := mock_scalers.NewMockScaler(ctrl)
	metricsSpecs := []v2.MetricSpec{createMetricSpec(averageValue, metricName)}

	metrics := []external_metrics.ExternalMetricValue{
		{
			MetricName: metricName,
			Value:      *resource.NewQuantity(queueLength, resource.DecimalSI),
		},
	}
	scaler.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs)
	scaler.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return(metrics, isActive, nil)
	scaler.EXPECT().Close(gomock.Any())
	return scaler
}

// -----------------------------------------------------------------------------
// test for scalingModifiers formula
// -----------------------------------------------------------------------------

const triggerName1 = "trigger_one"
const triggerName2 = "trigger_two"
const metricName1 = "metric_one"
const metricName2 = "metric_two"

func TestScalingModifiersFormula(t *testing.T) {
	scaledObjectName := testNameGlobal
	scaledObjectNamespace := testNamespaceGlobal
	compositeMetricName := compositeMetricNameGlobal

	ctrl := gomock.NewController(t)
	recorder := record.NewFakeRecorder(1)
	mockClient := mock_client.NewMockClient(ctrl)
	mockExecutor := mock_executor.NewMockScaleExecutor(ctrl)

	metricsSpecs1 := []v2.MetricSpec{createMetricSpec(2, metricName1)}
	metricsSpecs2 := []v2.MetricSpec{createMetricSpec(5, metricName2)}
	metricValue1 := scalers.GenerateMetricInMili(metricName1, float64(2))
	metricValue2 := scalers.GenerateMetricInMili(metricName2, float64(5))

	scaler1 := mock_scalers.NewMockScaler(ctrl)
	scaler2 := mock_scalers.NewMockScaler(ctrl)
	// dont use cached metrics
	scalerConfig1 := scalersconfig.ScalerConfig{TriggerUseCachedMetrics: false, TriggerName: triggerName1, TriggerIndex: 0}
	scalerConfig2 := scalersconfig.ScalerConfig{TriggerUseCachedMetrics: false, TriggerName: triggerName2, TriggerIndex: 1}
	factory1 := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		return scaler1, &scalerConfig1, nil
	}
	factory2 := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
		return scaler2, &scalerConfig2, nil
	}

	scaledObject := kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scaledObjectName,
			Namespace: scaledObjectNamespace,
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: "test",
			},
			Advanced: &kedav1alpha1.AdvancedConfig{
				ScalingModifiers: kedav1alpha1.ScalingModifiers{
					Target:  "2",
					Formula: fmt.Sprintf("%s + %s", triggerName1, triggerName2),
				},
			},
			Triggers: []kedav1alpha1.ScaleTriggers{
				{Name: triggerName1, Type: "fake_trig1"},
				{Name: triggerName2, Type: "fake_trig2"},
			},
		},
		Status: kedav1alpha1.ScaledObjectStatus{
			ScaleTargetGVKR: &kedav1alpha1.GroupVersionKindResource{
				Group: "apps",
				Kind:  "Deployment",
			},
			ExternalMetricNames: []string{metricName1, metricName2},
		},
	}

	// formula is compiled and cached
	compiledFormula, err := expr.Compile(scaledObject.Spec.Advanced.ScalingModifiers.Formula)
	assert.Equal(t, err, nil)

	scalerCache := cache.ScalersCache{
		ScaledObject: &scaledObject,
		Scalers: []cache.ScalerBuilder{{
			Scaler:       scaler1,
			ScalerConfig: scalerConfig1,
			Factory:      factory1,
		},
			{
				Scaler:       scaler2,
				ScalerConfig: scalerConfig2,
				Factory:      factory2,
			},
		},
		Recorder:        recorder,
		CompiledFormula: compiledFormula,
	}

	caches := map[string]*cache.ScalersCache{}
	caches[scaledObject.GenerateIdentifier()] = &scalerCache

	sh := scaleHandler{
		client:                   mockClient,
		scaleLoopContexts:        &sync.Map{},
		scaleExecutor:            mockExecutor,
		globalHTTPTimeout:        time.Duration(1000),
		recorder:                 recorder,
		scalerCaches:             caches,
		scalerCachesLock:         &sync.RWMutex{},
		scaledObjectsMetricCache: metricscache.NewMetricsCache(),
	}

	mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	scaler1.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs1)
	scaler2.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs2)
	scaler1.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{metricValue1, metricValue2}, true, nil)
	scaler2.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{metricValue1, metricValue2}, true, nil)
	mockExecutor.EXPECT().RequestScale(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	sh.checkScalers(context.TODO(), &scaledObject, &sync.RWMutex{})

	expectNoStatusPatch(ctrl)

	scaler1.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs1)
	scaler2.EXPECT().GetMetricSpecForScaling(gomock.Any()).Return(metricsSpecs2)
	scaler1.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{metricValue1, metricValue2}, true, nil)
	scaler2.EXPECT().GetMetricsAndActivity(gomock.Any(), gomock.Any()).Return([]external_metrics.ExternalMetricValue{metricValue1, metricValue2}, true, nil)
	metrics, err := sh.GetScaledObjectMetrics(context.TODO(), scaledObjectName, scaledObjectNamespace, compositeMetricName)
	assert.Nil(t, err)
	assert.Equal(t, float64(7), metrics.Items[0].Value.AsApproximateFloat64())
}

// createMetricSpec creates MetricSpec for given metric name and target value.
func createMetricSpec(averageValue int64, metricName string) v2.MetricSpec {
	qty := resource.NewQuantity(averageValue, resource.DecimalSI)
	return v2.MetricSpec{
		External: &v2.ExternalMetricSource{
			Target: v2.MetricTarget{
				AverageValue: qty,
			},
			Metric: v2.MetricIdentifier{
				Name: metricName,
			},
		},
	}
}

func expectNoStatusPatch(ctrl *gomock.Controller) {
	statusWriter := mock_client.NewMockStatusWriter(ctrl)
	statusWriter.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
}

/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"gitlab.com/pongsatt/githook/api/v1alpha1"
	githookclient "gitlab.com/pongsatt/githook/pkg/client"
	"gitlab.com/pongsatt/githook/pkg/githook"
	"gitlab.com/pongsatt/githook/pkg/model"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"net/url"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

func (r *GitHookReconciler) requestLogger(req ctrl.Request) logr.Logger {
	return r.Log.WithName(req.NamespacedName.String())
}

func ignoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *GitHookReconciler) sourceLogger(source *v1alpha1.GitHook) logr.Logger {
	return r.Log.WithName(fmt.Sprintf("%s/%s", source.Namespace, source.Name))
}

// GitHookReconciler reconciles a GitHook object
type GitHookReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	WebhookImage string
}

func getGitClient(source *v1alpha1.GitHook, options *model.HookOptions) (*githook.Client, error) {
	var gitClient githook.GitClient

	switch source.Spec.GitProvider {
	case v1alpha1.Gogs:
		gitClient = githookclient.NewGogsClient(options.BaseURL, options.AccessToken)
	case v1alpha1.Github:
		gitClient = githookclient.NewGithubClient(options.AccessToken)
	case v1alpha1.Gitlab:
		gitClient = githookclient.NewGitlabClient(options.BaseURL, options.AccessToken)
	default:
		return nil, fmt.Errorf("git provider %s not support", source.Spec.GitProvider)
	}

	return githook.New(gitClient, options.BaseURL, options.AccessToken)
}

//+kubebuilder:rbac:groups=tools.my.domain,resources=githooks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tools.my.domain,resources=githooks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tools.my.domain,resources=githooks/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GitHook object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *GitHookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//log := r.requestLogger(req)

	//log.Info("Reconciling " + req.NamespacedName.String())

	// Fetch the GitHook instance
	sourceOrg := &v1alpha1.GitHook{}
	err := r.Get(context.Background(), req.NamespacedName, sourceOrg)
	if err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, ignoreNotFound(err)
	}

	source := sourceOrg.DeepCopyObject()

	var reconcileErr error
	if sourceOrg.ObjectMeta.DeletionTimestamp == nil {
		reconcileErr = r.reconcile(source.(*v1alpha1.GitHook))
	}

	//log.Error(err, "Test to update")
	if err := r.Update(context.Background(), sourceOrg); err != nil {

		return ctrl.Result{}, err
	}
	return ctrl.Result{}, reconcileErr
}

func parseGitURL(gitURL string) (baseURL string, owner string, project string, err error) {
	u, err := url.Parse(gitURL)
	if err != nil {
		return "", "", "", err
	}

	paths := strings.Split(u.Path[1:], "/")
	baseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	owner = paths[0]
	project = paths[1]

	return baseURL, owner, project, nil
}

func (r *GitHookReconciler) buildHookFromSource(source *v1alpha1.GitHook) (*model.HookOptions, error) {
	hookOptions := &model.HookOptions{}

	baseURL, owner, projectName, err := parseGitURL(source.Spec.ProjectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to process project url to get the project name: " + err.Error())
	}

	hookOptions.BaseURL = baseURL
	hookOptions.Project = projectName
	hookOptions.Owner = owner
	hookOptions.ID = source.Status.ID

	for _, event := range source.Spec.EventTypes {
		hookOptions.Events = append(hookOptions.Events, string(event))
	}
	hookOptions.AccessToken, err = r.secretFrom(source.Namespace, source.Spec.AccessToken.SecretKeyRef)

	if err != nil {
		return nil, fmt.Errorf("failed to get accesstoken from secret %s/%s", source.Namespace, source.Spec.AccessToken.SecretKeyRef.Key)
	}

	hookOptions.SecretToken, err = r.secretFrom(source.Namespace, source.Spec.SecretToken.SecretKeyRef)

	if err != nil {
		return nil, fmt.Errorf("failed to get secret token from secret %s/%s", source.Namespace, source.Spec.AccessToken.SecretKeyRef.Key)
	}

	return hookOptions, nil
}

func (r *GitHookReconciler) reconcile(source *v1alpha1.GitHook) error {
	hookOptions, err := r.buildHookFromSource(source)
	if err != nil {
		return err
	}

	//TODO should be injected
	hookOptions.URL = "http://githook.com"

	hookID, err := r.reconcileWebhook(source, hookOptions)

	if err != nil {
		return err
	}

	source.Status.ID = hookID

	return nil
}

func (r *GitHookReconciler) reconcileWebhook(source *v1alpha1.GitHook, hookOptions *model.HookOptions) (string, error) {
	//log := r.sourceLogger(source)

	gitClient, err := getGitClient(source, hookOptions)

	if err != nil {
		return "", err
	}

	exists, changed, err := gitClient.Validate(hookOptions)

	if err != nil {
		return "", err
	}

	if !exists {
		//log.Info("create new webhook", "project", hookOptions.Project)
		hookID, err := gitClient.Create(hookOptions)

		if err != nil {
			return "", err
		}
		//log.Info("create new webhook successfully", "project", hookOptions.Project)
		return hookID, err
	}

	if err != nil {
		return "", err
	}

	if changed == true {
		//log.Info("update existing webhook", "project", hookOptions.Project)
		hookID, err := gitClient.Update(hookOptions)

		if err != nil {
			return "", err
		}

		//log.Info("update existing webhook successfully", "project", hookOptions.Project)

		return hookID, nil
	}

	//log.Info("webhook exists and updated", "project", hookOptions.Project)
	return hookOptions.ID, nil
}

func (r *GitHookReconciler) getSecret(namespace string, secretKeySelector *corev1.SecretKeySelector) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: secretKeySelector.Name}, secret)

	return secret, err
}

func (r *GitHookReconciler) secretFrom(namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret, err := r.getSecret(namespace, secretKeySelector)

	if err != nil {
		return "", err
	}
	secretVal, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return "", fmt.Errorf(`key "%s" not found in secret "%s"`, secretKeySelector.Key, secretKeySelector.Name)
	}

	return string(secretVal), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitHookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GitHook{}).
		Complete(r)
}

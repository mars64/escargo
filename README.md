# Disclaimer

For all intents, this is simply a capture of a previous solution I'd written a few times before the upstream argocdcd-image-updater behaved in the way I wanted. This exists purely for academic purposes. If you have need of a "last mile gitops" solution, please take a look at [argocd-image-updater](https://argocd-image-updater.readthedocs.io/en/stable/) instead! 

## Overview

`escArgo` is a CLI that automates "ArgoCD Write-Back" in a way that enables us to perform the write-back within a CI pipeline, based on the Gitlab web service. This is also what I call the "last mile gitops" problem.

`escArgo` will write a value to a given helm values path at a given file path.

## Usage

Given a helm values file such as:

```
product:
  image:
    repo: some-super-sweet-repo.dkr.aws.com
    tag: old-and-busted
```

The intention is to write a new value at a given tag path, such as `product.image.tag`. Run this from the root of the repository, and path accordingly.

e.g.:

```
% export GITLAB_TOKEN=<some_token>
% ./escargo -g=$GITLAB_TOKEN -f=apps/infrastructure-services/mars.dev.us-west-2.values.yaml -p=product.image.tag -n=new-hotness -d=false
```

should result in values file such as:

```
product:
  image:
    repo: some-super-sweet-repo.dkr.aws.com
    tag: new-hotness
```

We then expect the merge of this change to trigger a deployment.

## Caveats

The YAML parsing currently expects to reformat your yaml file. Please use caution, and see the associated [issue based on the current `mikefarah/yq` implementation](https://github.com/mikefarah/yq/issues/465).

## Order of Operation

One way this could work is, with a CI pipeline:

* build and push an image to a destination
* run `./escargo` with all the necessary flags

## Build

From the `/tools/escArgo` area of this repo:

```
% go build -o escargo .
```

## Dockerfile

All this stuff has been dockerized. Make sure you're [properly auth'ed to ecr](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html#cli-authenticate-registry), then:

```
% docker build -t escargo:latest some-super-sweet-repo.dkr.ecr.us-west-2.amazonaws.com/escargo:latest .
% docker image push some-super-sweet-repo.dkr.ecr.us-west-2.amazonaws.com/escargo:latest 
```

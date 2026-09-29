# images

Build files, one directory per deployed resource:

```
infra/images/<resource>/Containerfile
infra/images/<resource>/Containerfile.dockerignore
infra/images/shared/            # scripts used by more than one image
```

Images build from the repository root as context, so a Containerfile can copy
shared files. Reproduce a build locally with
`podman build -f infra/images/<resource>/Containerfile .`.

The same image runs in `next` and `prod`. Never bake environment-specific
values (URLs, credentials, feature flags) into an image; read them from the
environment at runtime.

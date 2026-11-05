# Automatic public frontend deployment

The native Gitea `verify` workflow runs the existing full repository verification,
then builds deployable images on PRs and main. PR image builds receive no registry
or deployment credentials. Main pushes and manual dispatches of main publish the
verified images through the existing organization registry credential, then
stream the same frontend image to the existing Almaz service.

This app uses Compose service `riot-public-web`, container `arkhiv-platform-public-20261004-riot-public-web-1`, at
https://riot-grrrl.subcult.tv/. Deployment uses this repository's dedicated `HOMELAB_DEPLOY_KEY` and
pinned `HOMELAB_KNOWN_HOSTS`. The forced-command receiver is maintained in the
private `subculture-collective/fleet-deploy` repository. Its local profile permits
only this app and frontend service. It refuses stale commits and wrong-app
images, retains the live Compose configuration, verifies image identity,
resources, runtime overrides and public HTML, and restores the previous image
if readiness fails. CI also compares the full public HTML with the image it
built. Private host receipts retain exact prior image IDs and rollback commands.

The homelab operator owns the key; its first rotation deadline is 2027-02-03.
Register and test the receiver/key before merging the pipeline. Keep prior
images, deployment files and receipts for recovery. Raw failure diagnostics stay
in mode-0600 host files and must not be attached to PRs or CI artifacts.

The shared core pins, database, media/archive mounts, public collection review,
original source metadata and engine release procedures retain their own
controls. This pipeline updates the frontend of the already public app; it does
not acquire content or publish additional material. Browser playback/reading,
audiovisual review, independent restore and deployment remain separate evidence.

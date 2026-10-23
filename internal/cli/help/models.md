# models

Usage: meno models [--provider NAME] [--refresh]

List the models of the enabled providers: the catalog plus the models each provider reports on /models.
--refresh asks the providers again; without it the cached list is used and, when empty, filled once.
Providers that cannot be asked are named on stderr.

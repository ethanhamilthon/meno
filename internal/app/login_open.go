package app

// openLogin shows the login list and reloads the OAuth accounts behind it.
func (a *App) openLogin() {
	a.openOverlay(OverLogin)
	a.refreshAccounts()
}

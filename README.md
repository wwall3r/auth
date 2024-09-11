# auth

This is heavily based on the [goth example](https://github.com/markbates/goth/blob/master/examples/main.go)

This got created because I hate spending time getting auth spun up within each 
side project. And a solution like Supabase is way too heavy (and eventually 
expensive) for my needs.

## Running locally

Note: most Oauth2 providers require a valid hostname for the setup on their end.
In dev, you can either use `http://lvh.me:<port>` (always verify it still points to
`127.0.0.1`) or use your hosts file to point something to `127.0.0.1`.

- Copy `.env.example` to `.env` and fill in those values
- `make install`
- `make dev`

## Integrations and Examples

Integrations with other languages are in the examples folder:

- [Go](/examples/go/)

## How it works

### Login

You have a login page which styles itself according to your app. It has a link
to each of the auth providers configured here.

```html
<a href="https://auth.example.com/auth/amazon">Amazon</a>
<a href="https://auth.example.com/auth/apple">Apple</a>
<a href="https://auth.example.com/auth/discord">Discord</a>
<a href="https://auth.example.com/auth/facebook">Facebook</a>
<a href="https://auth.example.com/auth/google">Google</a>
<a href="https://auth.example.com/auth/twitch">Twitch</a>
<a href="https://auth.example.com/auth/twitter">Twitter</a>
```

With each link, you should include a redirect URL:

```html
<a href="https://auth.example.com/auth/amazon?redirect=https%3A%2F%2Fexample.com%2Fuser%2Fprofile">Amazon</a>
```

(A good source of icons and brand colors for those links is [Simple Icons](https://simpleicons.org/))

When a user clicks on one of these links, they are redirected to the auth
provider's login page.

Once the user has logged in, they are redirected back to the redirect URL, with
two cookies via `gorilla/sessions` `Store` signed via `SESSION_SECRET`:

| Cookie | Key | Description |
| --- | --- | --- |
| user | `userId` | The user's ID |
| user | `email` | The user's email |
| long | `refreshToken` | The user's refresh token |
| long | `provider` | The provider the user last used to log in|

The `user` cookie expires after 30 days. The `long` cookie expires after a year.

The middleware for your app should either use `gorilla/sessions` with the same
`SESSION_SECRET` to pick those up ([see Go example](/examples/go)), or use the same method
in another language.

### Logout

Redirect or link to `https://auth.example.com/logout/{provider}?redirect=...`

The `provider` value in the `long` cookie will still be available to help indicate
to the user the provider they last used to log in.

### Redirect Param

The redirect URL must have the same scheme and apex domain as the auth host.

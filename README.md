# Switchyard
## What is Switchyard?
Switchyard is a dynamic HTTP virtualhost proxy router written in Go. 

It's a simple project intended to scratch an itch I have. I run far too many services with RESTful HTTP interfaces, often running an in-process server like Flask, Tornado, WEBBrick, whathaveyou. So quite a few ports on my home server are applications. Rather than remember which ports I assigned where, and on what machines (Say, :5000 for CouchPotato, :3001 for Tracks, but the Raspberry Pi hosts XBMC on port 8080 ... the list goes on) -- there's a better way. Virtualhosts.

Unfortunately, adding a virtualhost in nginx or Apache would require adding it to the config and reloading the config, potentially restarting the webserver. This seems like a bit much for low-traffic things like development/personal sites.

With the rise of [docker](http://docker.io) more HTTP services are forthcoming on odd ports. So I wanted a simple way to add a virtualhost in one curl command.

Enter Switchyard. 

(You might want to understand routing, vhosts, and DNS before continuing. This is a hacker's project.)

## How does it work?

Suppose my domain is `example.com`

It's easy enough to get a home router to route all things bound for `*.app.example.com` to go to one server. If you use DNSMasq like DD-WRT or Tomato do, the magic line looks something like:

```
address=/.app.example.com/192.168.0.2
```

If your home server's IP is 192.168.0.2

Then, there are two options --

1) Run Switchyard on port 80, and route appropriately inside Switchyard.
2) Run Switchyard on the default 8888 and have Apache or nginx have only one virtualhost route for all things bound to *.app.example.com to port 8888

Once a request makes it to Switchyard, if there's a valid route for the hostname, it will proxy the request to the desired location.

## How do you build Switchyard?

Obviously, Go is required. I wrote it against 1.1, not sure if it works with anything older. It also steals an idea from Python virtualenvs (though switchyard is simple enough not to need dependencies, it may someday)

To wit:
```
git clone http://github.com/barakmich/switchyard
cd switchyard
source activate.sh
go build switchyard
./switchyard
```

And you're off to the races. Two ports will open up -- one is the router port (default 8888, where HTTP requests get routed from) and one is a configuration port (default 8889).  The next step is obvious -- open the configuration port in a browser.

On the page, you'll be able to add a route. If you've successfully punched the holes to get requests into Switchyard's listening port, the next step is actually kind of cute. Add "switchyard.app.example.com" to route to "127.0.0.1:8889" -- your first route can be the server itself, for further configuration. If you then browse over to "switchyard.app.example.com" and see the configuration screen, success! You have a working switchyard.

## How do you configure Switchyard?

There are only a few options available at this time

```
--port=8888 -- The port which gets HTTP requests for routing
--cfg_port=8889 -- The port for the HTTP frontend for configuring Switchyard
--route_file=switchyard.csv -- The save file for your routes, defaults to switchyard.csv in the current directory.
```

The rest is all available on the configuration interface.

## How do I dynamically add switches?

If I'm scripting something from bash, a simple way (once you have the first route) is to just:

```
curl "http://switchyard.app.example.com/add?host=$VHOSTNAME&target=$TARGETPORT" > /dev/null
```

For values of VHOSTNAME and TARGETPORT -- discernable from whatever your usecase is.

## Licensing
Some credit goes to Andreas Krennmair who wrote [akrennmair/drunken-hipster](http://github.com/akrennmair/drunken-hipster), which I cribbed from and simplified. As he graciously licensed his code under MIT, my additions/modifications are also MIT (even if I usually go Simplified BSD) -- which is how the open source community thrives. This is more a happy forked branch than a licensing takeover. Both copyrights appear in the LICENSE.

## Contact Me
[Follow me on Twitter](http://twitter.com/barakmich) or otherwise write me a nice message. Pull requests encouraged!

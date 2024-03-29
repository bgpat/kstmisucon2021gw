HOSTNAME := $(shell hostname)

.PHONY: all
all: git ssh commit /home/ishocon/.bashrc /files/hosts/$(HOSTNAME)
	@echo "\n\n\n\n"
	@echo "Open \e[4mhttps://github.com/$$(\
		git config --get remote.origin.url | sed -r 's/^.*?:(.*)\.git$$/\1/' \
	)/settings/keys/new\e[0m and paste pubkey:\n"
	@cat /root/.ssh/id_rsa.pub
	@echo

.PHONY: commit
commit:
	git add -A
	git commit -m "Configure $(HOSTNAME)"

.PHONY: git
git: /.gitignore /usr/local/bin/git-preserve-permissions /root/.vimrc /home/ishocon/.vimrc
	git config --get remote.origin.url \
	| grep '^https://' \
	&& git config --get remote.origin.url \
	| sed -r 's%^https://github.com/([a-zA-Z0-9_-]*/[a-zA-Z0-9_-]*).*$$%git@github.com:\1.git%' \
	| xargs git remote set-url origin \
	|| true

.PHONY: ssh
ssh: /root/.ssh/authorized_keys /files/pubkey/$(HOSTNAME) sshd

.PHONY: sshd
sshd: /etc/sudoers /etc/ssh/sshd_config

.PHONY: /etc/sudoers
/etc/sudoers: /etc/sudoers.bak
	echo 'Defaults	env_keep+="GIT_AUTHOR_NAME"' >> $@
	echo 'Defaults	env_keep+="GIT_AUTHOR_EMAIL"' >> $@

/etc/sudoers.bak:
	cp -p /etc/sudoers $@

.PHONY: /etc/ssh/sshd_config
/etc/ssh/sshd_config: /etc/ssh/sshd_config.bak ssh_host_key
	sed -i '/^PermitRootLogin no/d' $@
	echo 'PermitUserEnvironment yes' >> $@
	echo 'PermitRootLogin without-password' >> $@
	sshd -t
	systemctl reload sshd

/etc/ssh/sshd_config.bak:
	cp -p /etc/ssh/sshd_config $@

.PHONY: clean
clean:
	rm -rf /root/.ssh /.gitignore /root/.vimrc /home/ishocon/.vimrc /root/.ssh/id_rsa /root/.gitconfig
	test -e /etc/sudoers.bak && cp -fp /etc/sudoers.bak /etc/sudoers
	test -e /etc/ssh/sshd_config.bak && cp -fp /etc/ssh/sshd_config.bak /etc/ssh/sshd_config

/root/.ssh:
	mkdir -p /root/.ssh
	chmod 700 /root/.ssh

/root/.ssh/authorized_keys: /root/.ssh
	touch $@
	chmod 600 $@
	git add -f $@

/.gitignore: files/gitignore
	cp -n $< $@

/root/.vimrc: files/vimrc
	cp -f $< $@

/home/ishocon/.vimrc: files/vimrc
	cp -f $< $@

/root/.ssh/id_rsa: /root/.ssh
	yes | ssh-keygen -f $@ -t rsa -N "" -b 4096 > /dev/null

/root/.ssh/id_rsa.pub: /root/.ssh/id_rsa

/files/pubkey/$(HOSTNAME): /root/.ssh/id_rsa.pub
	mkdir -p /files/pubkey
	cp -f $< $@
	git add -f $@

/root/.gitconfig:
	git config --global user.email "anonymous@example.com"
	git config --global user.name "anonymous"
	git config --global preserve-permissions.user "true"
	git config --global preserve-permissions.group "true"

.PHONY: ssh_host_key
ssh_host_key:
	ssh-keygen -A

/usr/local/bin/git-preserve-permissions: /git-preserve-permissions /root/.gitconfig
	cp -f $</git-preserve-permissions $@
	cp -f $</post-checkout $</post-merge $</pre-commit /.git/hooks/
	-git preserve-permissions --save

/git-preserve-permissions:
	git clone --depth=1 https://github.com/dr4Ke/git-preserve-permissions.git

/home/ishocon:
	mkdir -p $@

/home/ishocon/.bashrc.backup: /home/ishocon
	cp -f /home/ishocon/.bashrc $@
	/files/gitignore.sh $@

/home/ishocon/.bashrc: /home/ishocon/.bashrc.backup
ifeq ($(shell git status --ignored --short /home/ishocon/.bashrc),)
	@echo skip $@
else
	/files/gitignore.sh $@
	echo 'alias git="sudo git"' >> $@
	git add -f $@
	git commit -m 'Add alias for git'
endif

/files/hosts/$(HOSTNAME):
	mkdir -p /files/hosts
	echo "$(shell hostname -s) $(shell hostname -i)" > $@
	-git add -f $@
	-git commit -m 'Add host'

.PHONY: monitoring
monitoring: prometheus node_exporter process-exporter

.PHONY: prometheus
prometheus: /usr/local/bin/prometheus
	-pkill $@
	nohup $@ --config.file=/etc/prometheus/prometheus.yml &

/usr/local/bin/prometheus:
	curl -sL https://github.com/prometheus/prometheus/releases/download/v2.21.0-rc.1/prometheus-2.21.0-rc.1.linux-amd64.tar.gz | tar xzv --strip-components 1 -C /usr/local/bin/ prometheus-2.21.0-rc.1.linux-amd64/prometheus

.PHONY: node_exporter
node_exporter: /usr/local/bin/node_exporter
	-pkill $@
	nohup $@ &

/usr/local/bin/node_exporter:
	curl -sL https://github.com/prometheus/node_exporter/releases/download/v1.0.1/node_exporter-1.0.1.linux-amd64.tar.gz | tar xzv --strip-components 1 -C /usr/local/bin/ node_exporter-1.0.1.linux-amd64/node_exporter

.PHONY: process-exporter
process-exporter: /usr/local/bin/process-exporter /etc/process-exporter/all.yaml
	-pkill $@
	nohup $@ -config.path /etc/process-exporter/all.yaml &

/usr/local/bin/process-exporter:
	curl -sL https://github.com/ncabatoff/process-exporter/releases/download/v0.7.2/process-exporter-0.7.2.linux-amd64.tar.gz | tar xzv --strip-components 1 -C /usr/local/bin/ process-exporter-0.7.2.linux-amd64/process-exporter

/etc/process-exporter/all.yaml: /etc/process-exporter
	curl -sL https://raw.githubusercontent.com/ncabatoff/process-exporter/v0.7.2/packaging/conf/all.yaml > $@

/etc/process-exporter:
	mkdir -p $@

.PHONY: grafana-server
grafana-server: /usr/local/bin/grafana-server
	-pkill $@
	GF_AUTH_ANONYMOUS_ENABLED=true GF_AUTH_ANONYMOUS_ORG_ROLE=Admin nohup $@ -homepath /opt/grafana &

/usr/local/bin/grafana-server: /opt/grafana
	ln -sf $</bin/grafana-server $@

/opt/grafana:
	mkdir -p $@
	curl -sL https://dl.grafana.com/oss/release/grafana-7.5.5.linux-amd64.tar.gz | tar xzv --strip-components 1 -C $@
	cp -f /files/grafana/datasources.yml $@/conf/provisioning/datasources/datasources.yml
	cp -f /files/grafana/dashboards.yml $@/conf/provisioning/dashboards/dashboards.yml

/usr/local/bin/jaeger-all-in-one:
	curl -sL https://github.com/jaegertracing/jaeger/releases/download/v1.22.0/jaeger-1.22.0-linux-amd64.tar.gz | tar xzv --strip-components 1 -C /usr/local/bin/ jaeger-1.22.0-linux-amd64/jaeger-all-in-one

.PHONY: jeager
jaeger: /usr/local/bin/jaeger-all-in-one
	-pkill jaeger-all-in-one
	SPAN_STORAGE_TYPE=memory nohup jaeger-all-in-one --memory.max-traces 100 &

slow.log: /tmp/mysql-slow.sql
	mysqldumpslow -s t $< > $@
	cat $@

/usr/local/bin/kataribe:
	wget -q -O /tmp/kataribe-v0.4.1_linux_amd64.zip https://github.com/matsuu/kataribe/releases/download/v0.4.1/kataribe-v0.4.1_linux_amd64.zip
	unzip /tmp/kataribe-v0.4.1_linux_amd64.zip -d /usr/local/bin
	rm -rf /tmp/kataribe-v0.4.1_linux_amd64.zip

kataribe.log: /var/log/nginx/access.log kataribe.toml /usr/local/bin/kataribe
	/usr/local/bin/kataribe -f /kataribe.toml < $< > kataribe.log
	cat $@

.PHONY: rotate
rotate:
	rm -f /tmp/mysql-slow.sql /var/log/nginx/access.log /var/log/nginx/error.log
	systemctl restart mysql nginx

.PHONY: deploy
deploy: go1.16.3
	systemctl stop webapp
	rm -f /home/ishocon/webapp/go/webapp
	$(MAKE) /home/ishocon/webapp/go/webapp
	systemctl start webapp
	systemctl status webapp

/home/ishocon/webapp/go/webapp: /home/ishocon/webapp/go/*.go
	cd /home/ishocon/webapp/go && go build -o webapp .

go1.16.3.linux-amd64.tar.gz:
	wget https://golang.org/dl/go1.16.3.linux-amd64.tar.gz

.PHONY: go1.16.3
go1.16.3: go1.16.3.linux-amd64.tar.gz
	rm -rf /usr/local/go && tar -C /usr/local -xzf go1.16.3.linux-amd64.tar.gz

#!/bin/bash
mysql -u ishocon -pishocon ishocon1 < ~/.init/init.sql
ruby ~/.init/insert.rb

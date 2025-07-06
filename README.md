sudo chmod 644 /etc/systemd/system/twiliosms.service
sudo systemctl daemon-reload
sudo systemctl enable twiliosms.service
sudo systemctl start twiliosms.service
sudo systemctl status twiliosms.service




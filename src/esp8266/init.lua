station_cfg={}
-- 无线路由设置
station_cfg.ssid="HOMEWIFI"
station_cfg.pwd="HOMEPASS"
station_cfg.save=true
wifi.setmode(wifi.STATION)
wifi.sta.config(station_cfg)
-- ESP8266 led pin
pin = 4
gpio.mode(pin, gpio.OUTPUT)
gpio.write(pin, gpio.HIGH)



-- Serving static files
dofile('httpServer.lua')
httpServer:listen(80)


httpServer:use('/temp', function(req, res)
	status, temp, humi, temp_dec, humi_dec = dht.read(1)
	if status == dht.OK then
		res:send(string.format("温度: %d\r\n", temp, humi))
	end
end)

httpServer:use('/humi', function(req, res)
	status, temp, humi, temp_dec, humi_dec = dht.read(1)
	if status == dht.OK then
		res:send(string.format("湿度: %d\r\n", humi))
	end
end)

httpServer:use('/led/on', function(req, res)
    gpio.write(pin, gpio.LOW)
	res:send("LED ON\r\n")
end)
httpServer:use('/led/off', function(req, res)
    gpio.write(pin, gpio.HIGH)
	res:send("LED OFF\r\n")
end)


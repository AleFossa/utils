package iputil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	stringsutil "github.com/projectdiscovery/utils/strings"
	"go.uber.org/multierr"
)

// internalRangeChecker contains a list of internal IP ranges.
type internalRangeChecker struct {
	ipv4 []*net.IPNet
	ipv6 []*net.IPNet
}

var (
	ipv4InternalRanges = []string{}
	ipv6InternalRanges = []string{}
	rangeChecker       = internalRangeChecker{}
)

func init() {
	// ipv4InternalRanges contains the IP ranges internal in IPv4 range.
	ipv4InternalRanges = []string{
		"0.0.0.0/8",       // Current network (only valid as source address)
		"10.0.0.0/8",      // Private network
		"100.64.0.0/10",   // Shared Address Space
		"127.0.0.0/8",     // Loopback
		"169.254.0.0/16",  // Link-local (Also many cloud providers Metadata endpoint)
		"172.16.0.0/12",   // Private network
		"192.0.0.0/24",    // IETF Protocol Assignments
		"192.0.2.0/24",    // TEST-NET-1, documentation and examples
		"192.88.99.0/24",  // IPv6 to IPv4 relay (includes 2002::/16)
		"192.168.0.0/16",  // Private network
		"198.18.0.0/15",   // Network benchmark tests
		"198.51.100.0/24", // TEST-NET-2, documentation and examples
		"203.0.113.0/24",  // TEST-NET-3, documentation and examples
		"224.0.0.0/4",     // IP multicast (former Class D network)
		"240.0.0.0/4",     // Reserved (former Class E network)
	}

	// ipv6InternalRanges contains the IP ranges internal in IPv6 range.
	ipv6InternalRanges = []string{
		"::1/128",       // Loopback
		"64:ff9b::/96",  // IPv4/IPv6 translation (RFC 6052)
		"100::/64",      // Discard prefix (RFC 6666)
		"2001::/32",     // Teredo tunneling
		"2001:10::/28",  // Deprecated (previously ORCHID)
		"2001:20::/28",  // ORCHIDv2
		"2001:db8::/32", // Addresses used in documentation and example source code
		"2002::/16",     // 6to4
		"fc00::/7",      // Unique local address
		"fe80::/10",     // Link-local address
		"ff00::/8",      // Multicast
	}

	_, err := newInternalRangeChecker()
	if err != nil {
		log.Fatal(err)
	}
}

// IsIP checks if a string is either IP version 4 or 6. Alias for `net.ParseIP`
func IsIP(str string) bool {
	return net.ParseIP(str) != nil
}

// IsPort checks if a string represents a valid port
func IsPort(str string) bool {
	if i, err := strconv.Atoi(str); err == nil && i > 0 && i < 65536 {
		return true
	}
	return false
}

// IsIPv4 checks if the string is an IP version 4.
func IsIPv4(ips ...interface{}) bool {
	for _, ip := range ips {
		switch ipv := ip.(type) {
		case string:
			parsedIP := net.ParseIP(ipv)
			isIP4 := parsedIP != nil && parsedIP.To4() != nil && stringsutil.ContainsAny(ipv, ".")
			if !isIP4 {
				return false
			}
		case net.IP:
			isIP4 := ipv != nil && ipv.To4() != nil && stringsutil.ContainsAny(ipv.String(), ".")
			if !isIP4 {
				return false
			}
		}
	}

	return true
}

// Check if an IP address is part of the list of internal IPs we have declared
// checks for all ipv4 and ipv6 list
func IsInternal(str string) bool {
	if !IsIP(str) {
		return false

	}
	IP := net.ParseIP(str)
	for _, net := range rangeChecker.ipv4 {
		if net.Contains(IP) {
			return true
		}
	}
	for _, net := range rangeChecker.ipv6 {
		if net.Contains(IP) {
			return true
		}
	}
	return false
}

// Check if an IP address is part of the list of internal IPs we have declared
func IsInIpv4List(str string) bool {
	for _, ip := range ipv4InternalRanges {
		if strings.Contains(ip, str) {
			return true
		}
	}
	return false
}

// Check if an IP address is part of the list of internal IPs we have declared
func IsInIpv6List(str string) bool {
	for _, ip := range ipv6InternalRanges {
		if strings.Contains(ip, str) {
			return true
		}
	}
	return false
}

// IsIPv6 checks if the string is an IP version 6.
func IsIPv6(ips ...interface{}) bool {
	for _, ip := range ips {
		switch ipv := ip.(type) {
		case string:
			parsedIP := net.ParseIP(ipv)
			isIP6 := parsedIP != nil && parsedIP.To16() != nil && stringsutil.ContainsAny(ipv, ":")
			if !isIP6 {
				return false
			}
		case net.IP:
			isIP6 := ipv != nil && ipv.To16() != nil && stringsutil.ContainsAny(ipv.String(), ":")
			if !isIP6 {
				return false
			}
		}
	}

	return true
}

// IsCIDR checks if the string is an valid CIDR notiation (IPV4 & IPV6)
func IsCIDR(str string) bool {
	_, _, err := net.ParseCIDR(str)
	return err == nil
}

// IsCIDR checks if the string is an valid CIDR after replacing - with /
func IsCidrWithExpansion(str string) bool {
	str = strings.ReplaceAll(str, "-", "/")
	return IsCIDR(str)
}

// ToCidr converts a cidr string to net.IPNet pointer
func ToCidr(item string) *net.IPNet {
	if IsIPv4(item) {
		item += "/32"
	} else if IsIPv6(item) {
		item += "/128"
	}
	if IsCIDR(item) {
		_, ipnet, _ := net.ParseCIDR(item)
		// a few ip4 might be expressed as ip6, therefore perform a double conversion
		_, ipnet, _ = net.ParseCIDR(ipnet.String())
		return ipnet
	}

	return nil
}

// AsIPV4CIDR converts ipv4 cidr to net.IPNet pointer
func AsIPV4IpNet(IPV4 string) *net.IPNet {
	if IsIPv4(IPV4) {
		IPV4 += "/32"
	}
	_, network, err := net.ParseCIDR(IPV4)
	if err != nil {
		return nil
	}
	return network
}

// AsIPV6IpNet converts ipv6 cidr to net.IPNet pointer
func AsIPV6IpNet(IPV6 string) *net.IPNet {
	if IsIPv6(IPV6) {
		IPV6 += "/64"
	}
	_, network, err := net.ParseCIDR(IPV6)
	if err != nil {
		return nil
	}
	return network
}

// AsIPV4CIDR converts ipv4 ip to cidr string
func AsIPV4CIDR(IPV4 string) string {
	if IsIP(IPV4) {
		return IPV4 + "/32"
	}
	return IPV4
}

// AsIPV4CIDR converts ipv6 ip to cidr string
func AsIPV6CIDR(IPV6 string) string {
	// todo
	return IPV6
}

// WhatsMyIP attempts to obtain the external ip through public api
// Copied from https://github.com/projectdiscovery/naabu/blob/master/v2/pkg/scan/externalip.go
func WhatsMyIP() (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.ipify.org?format=text", nil)
	if err != nil {
		return "", nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error fetching ip: %s", resp.Status)
	}

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(ip), nil
}

// GetSourceIP gets the local ip based the destination ip
func GetSourceIP(target string) (net.IP, error) {
	hostPort := net.JoinHostPort(target, "12345")
	serverAddr, err := net.ResolveUDPAddr("udp", hostPort)
	if err != nil {
		return nil, err
	}

	con, dialUpErr := net.DialUDP("udp", nil, serverAddr)
	if dialUpErr != nil {
		return nil, dialUpErr
	}

	defer con.Close()

	if udpaddr, ok := con.LocalAddr().(*net.UDPAddr); ok {
		return udpaddr.IP, nil
	}

	return nil, errors.New("could not get source ip")
}

// GetBindableAddress on port p from a list of ips
func GetBindableAddress(port int, ips ...string) (string, error) {
	var errs error
	// iterate over ips and return the first bindable one on port p
	for _, ip := range ips {
		if ip == "" {
			continue
		}
		ipPort := net.JoinHostPort(ip, fmt.Sprint(port))
		// check if we can listen on tcp
		l, err := net.Listen("tcp", ipPort)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		l.Close()
		udpAddr := net.UDPAddr{
			Port: port,
			IP:   net.ParseIP(ip),
		}
		// check if we can listen on udp
		lu, err := net.ListenUDP("udp", &udpAddr)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		lu.Close()

		// we found a bindable ip
		return ip, nil
	}

	return "", errs
}

// newInternalRangeChecker creates a structure for checking if a host is from
// a internal IP range whether its ipv4 or ipv6.
func newInternalRangeChecker() (*internalRangeChecker, error) {
	err := rangeChecker.appendIPv4Ranges(ipv4InternalRanges)
	if err != nil {
		return nil, err
	}

	err = rangeChecker.appendIPv6Ranges(ipv6InternalRanges)
	if err != nil {
		return nil, err
	}
	return &rangeChecker, nil
}

// appendIPv4Ranges adds a list of IPv4 Ranges to the list.
func (r *internalRangeChecker) appendIPv4Ranges(ranges []string) error {
	for _, ip := range ranges {
		_, rangeNet, err := net.ParseCIDR(ip)
		if err != nil {
			return err
		}
		r.ipv4 = append(r.ipv4, rangeNet)
	}
	return nil
}

// appendIPv6Ranges adds a list of IPv6 Ranges to the list.
func (r *internalRangeChecker) appendIPv6Ranges(ranges []string) error {
	for _, ip := range ranges {
		_, rangeNet, err := net.ParseCIDR(ip)
		if err != nil {
			return err
		}
		r.ipv6 = append(r.ipv6, rangeNet)
	}
	return nil
}

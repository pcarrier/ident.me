import SwiftUI
import MapKit

struct InternalIP: Identifiable {
    let interface: String
    let ip: String

    var id: String { "\(interface):\(ip)" }
}

func getInternalIPs() -> ([InternalIP], [InternalIP])? {
    var v4 = [InternalIP]()
    var v6 = [InternalIP]()

    var ifaddr : UnsafeMutablePointer<ifaddrs>?
    guard getifaddrs(&ifaddr) == 0 else { return nil }
    guard let firstAddr = ifaddr else { return nil }

    for ifptr in sequence(first: firstAddr, next: { $0.pointee.ifa_next }) {
        let interface = ifptr.pointee
        let addr = interface.ifa_addr.pointee
        let name = String(cString: interface.ifa_name)
        if name == "lo0" {
            continue
        }
        var hostname = [CChar](repeating: 0, count: Int(NI_MAXHOST))
        if (getnameinfo(interface.ifa_addr, socklen_t(addr.sa_len), &hostname, socklen_t(hostname.count), nil, socklen_t(0), NI_NUMERICHOST) == 0) {
            let address = String(cString: hostname)
            if address.contains("%") {
                continue
            }
            switch (addr.sa_family) {
            case UInt8(AF_INET):
                v4.append(InternalIP(interface: name, ip: address))
            case UInt8(AF_INET6):
                v6.append(InternalIP(interface: name, ip: address))
            default:
                continue
            }
        }
    }

    freeifaddrs(ifaddr)
    return (v4, v6)
}

struct Ident: Codable {
    let ip: String
    let aso: String
    let asn: String
    let postal: String
    let city: String
    let country: String
    let latitude: String
    let longitude: String

    func loc() -> String {
        [postal, city, country].filter { str in !str.isEmpty }.joined(separator: ", ")
    }
}

final class IdentModel : ObservableObject {
    private let v4url = URL(string: "https://4.ident.me/json")!
    private let v6url = URL(string: "https://6.ident.me/json")!

    @Published var refreshing: Int = 0
    @Published var v4: (Ident?, String?) = (nil, nil)
    @Published var v6: (Ident?, String?) = (nil, nil)
    @Published var internalIPs: ([InternalIP], [InternalIP]) = ([], []);
    @Published var fetch: (Date, Double)? = nil

    init() {
        refresh()
    }

    func fetchedStr() -> String? {
        guard let fetch = fetch else { return nil }
        let dateFormatter = DateFormatter()
        dateFormatter.dateStyle = .long
        dateFormatter.timeStyle = .medium
        return "\(dateFormatter.string(from: fetch.0)) (\(String(format: "%.3f", fetch.1))s)"
    }

    func refresh() {
        refreshing = 2
        let started = Date()

        if let ips = getInternalIPs() {
            internalIPs = ips;
        }

        let decoder = JSONDecoder()

        URLSession.shared.dataTask(with: v4url) { data, response, error in
            DispatchQueue.main.async {
                if let data = data {
                    if let ident = try? decoder.decode(Ident.self, from: data) {
                        self.v4 = (ident, nil)
                    }
                } else if let error = error {
                    self.v4 = (nil, "Error: \(error.localizedDescription)")
                }
                self.refreshing -= 1
                if (self.refreshing == 0) {
                    let now = Date()
                    self.fetch = (now, now.timeIntervalSince(started))
                }
            }
        }.resume()
        URLSession.shared.dataTask(with: v6url) { data, response, error in
            DispatchQueue.main.async {
                if let data = data {
                    if let ident = try? decoder.decode(Ident.self, from: data) {
                        self.v6 = (ident, nil)
                    }
                } else if let error = error {
                    self.v6 = (nil, "Error: \(error.localizedDescription)")
                }
                self.refreshing -= 1
                if (self.refreshing == 0) {
                    let now = Date()
                    self.fetch = (now, now.timeIntervalSince(started))
                }
            }
        }.resume()
    }
}

struct IntervalView: View {
    var model: [InternalIP]

    var body: some View {
        ForEach(model) { entry in
            GridRow {
                HStack {
                    Text("\(entry.ip)")
                        .monospaced()
                        .frame(maxWidth: .infinity, alignment: .leading)
                    Text("(\(entry.interface))")
                }
                Button {
                    #if os(OSX)
                        NSPasteboard.general.setString(entry.ip, forType: .string)
                    #else
                        UIPasteboard.general.string = entry.ip
                    #endif
                } label: {
                    Image(systemName: "clipboard")
                    Text("Copy")
                }.buttonStyle(.bordered)
            }
        }
    }
}

struct IdentView: View {
    var model: Ident?
    var msg: String?

    var body: some View {
        if let msg = msg {
            Text(msg)
                .italic()
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.bottom)
        } else if let model = model {
            GridRow {
                Text(model.ip)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .monospaced()
                Button {
                    #if os(OSX)
                        NSPasteboard.general.setString(model.ip, forType: .string)
                    #else
                        UIPasteboard.general.string = model.ip
                    #endif
                } label: {
                    Image(systemName: "clipboard")
                    Text("Copy")
                }.buttonStyle(.bordered)
            }
            GridRow {
                Text(model.loc())
                    .frame(maxWidth: .infinity, alignment: .leading)
                if let lat = Double(model.latitude), let lon = Double(model.longitude) {
                    Button {
                        MKMapItem(placemark: MKPlacemark(coordinate: CLLocationCoordinate2DMake(lat, lon))).openInMaps()
                    } label: {
                        Image(systemName: "map")
                        Text("Spot")
                    }.buttonStyle(.bordered)
                }
            }
            GridRow {
                Text("\(model.aso) (\(model.asn))")
                    .frame(maxWidth: .infinity, alignment: .leading)
                Button {
                    let url = URL(string: "https://bgpview.io/asn/\(model.asn)")!
                    #if os(OSX)
                        NSWorkspace.shared.open(url)
                    #else
                        UIApplication.shared.open(url)
                    #endif
                } label: {
                    Image(systemName: "network")
                    Text("Infos")
                }.buttonStyle(.bordered)
            }
        }
    }
}

struct ContentView: View {
    @StateObject var viewModel = IdentModel()

    var body: some View {
        TabView {
            ScrollView {
                Grid {
                    VStack {
                        Image(systemName: "globe")
                            .font(.system(size: 64))
                        Text("Public IPs")
                            .font(.title)
                        Text("identify your device on the Internet")
                            .padding(.bottom)
                    }

                    Text("IPv4")
                        .font(.title2)
                    IdentView(model: viewModel.v4.0, msg: viewModel.v4.1)
                        .padding(.bottom)

                    Text("IPv6")
                        .font(.title2)

                    IdentView(model: viewModel.v6.0, msg: viewModel.v6.1)
                        .padding(.bottom)

                    if let fetched = viewModel.fetchedStr() {
                        Text("refreshed \(fetched)")
                            .font(.footnote)
                            .padding(.bottom)
                    }

                    Button {
                        viewModel.refresh()
                    } label: {
                        Image(systemName: "arrow.clockwise")
                        Text("Refresh")
                    }
                    .disabled(viewModel.refreshing != 0)
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
                }.padding()
            }
            .tabItem {
                Label("Public IPs", systemImage: "globe")
            }
            ScrollView {
                Grid {
                    VStack {
                        Image(systemName: "wifi")
                            .font(.system(size: 64))
                        Text("Local IPs")
                            .font(.title)
                        Text("identify your device on local networks")
                            .padding(.bottom)
                    }

                    Text("IPv4")
                        .font(.title2)
                    IntervalView(model: viewModel.internalIPs.0)
                        .padding(.bottom)

                    Text("IPv6")
                        .font(.title2)
                    IntervalView(model: viewModel.internalIPs.1)
                        .padding(.bottom)

                    if let fetched = viewModel.fetchedStr() {
                        Text("refreshed \(fetched)")
                            .font(.footnote)
                            .padding(.bottom)
                    }

                    Button {
                        viewModel.refresh()
                    } label: {
                        Image(systemName: "arrow.clockwise")
                        Text("Refresh")
                    }
                    .disabled(viewModel.refreshing != 0)
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
                }.padding()
            }
            .tabItem {
                Label("Local IPs", systemImage: "wifi")
            }
        }
    }
}

struct ContentView_Previews: PreviewProvider {
    static var previews: some View {
        ContentView()
    }
}

@main
struct identMeApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
                .frame(idealWidth: 400, idealHeight: 400)
        }
    }
}

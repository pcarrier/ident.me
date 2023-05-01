import SwiftUI
import MapKit

struct Ident: Codable {
    let ip: String
    let aso: String
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
    @Published var fetchStarted: Date? = nil
    @Published var fetchEnded: Date? = nil
    
    init() {
        refresh()
    }
    
    func fetchedStr() -> String? {
        if let ended = fetchEnded {
            let started = fetchStarted!
            let dateFormatter = DateFormatter()
            dateFormatter.dateStyle = .long
            dateFormatter.timeStyle = .medium
            return "\(dateFormatter.string(from: ended)) (\(String(format: "%.3f", ended.timeIntervalSince(started)))s)"
        } else {
            return nil
        }
    }

    func refresh() {
        refreshing = 2
        fetchStarted = Date()
        
        let decoder = JSONDecoder()
        
        URLSession.shared.dataTask(with: v4url) { data, response, error in
            DispatchQueue.main.async {
                if let data = data {
                    if let ident = try? decoder.decode(Ident.self, from: data) {
                        self.v4 = (ident, nil)
                    }
                } else if let error = error {
                    self.v4 = (nil, error.localizedDescription)
                }
                self.refreshing -= 1
                if (self.refreshing == 0) {
                    self.fetchEnded = Date()
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
                    self.v6 = (nil, error.localizedDescription)
                }
                self.refreshing -= 1
                if (self.refreshing == 0) {
                    self.fetchEnded = Date()
                }
            }
        }.resume()
    }
}

struct IdentView: View {
    var model: Ident?
    var msg: String?
    
    var body: some View {
        if let msg = msg {
            Text(msg).italic().padding(.bottom)
        } else if let model = model {
            GridRow {
                Text(model.ip).multilineTextAlignment(.center).monospaced()
                Button {
                    UIPasteboard.general.string = model.ip
                } label: {
                    Image(systemName: "clipboard")
                    Text("Copy")
                }.buttonStyle(.bordered)
            }
            GridRow {
                Text("\(model.loc()) (\(model.aso))").font(.footnote).multilineTextAlignment(.center)
                if let lat = Double(model.latitude), let lon = Double(model.longitude) {
                    Button {
                        MKMapItem(placemark: MKPlacemark(coordinate: CLLocationCoordinate2DMake(lat, lon))).openInMaps()
                    } label: {
                        Image(systemName: "map")
                        Text("Spot")
                    }.buttonStyle(.bordered)
                }
            }.padding(.bottom)
        }
    }
}

struct ContentView: View {
    @StateObject var viewModel = IdentModel()

    var body: some View {
        Grid {
            Text("IPv4").font(.headline)
            IdentView(model: viewModel.v4.0, msg: viewModel.v4.1)
            
            Text("IPv6").font(.headline)
            IdentView(model: viewModel.v6.0, msg: viewModel.v6.1)
            
            if let fetched = viewModel.fetchedStr() {
                Text("fetched \(fetched)").font(.footnote).padding(.bottom).italic()
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
        }
    }
}

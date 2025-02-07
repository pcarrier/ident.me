#include <string>
#include <iostream>
#include <vector>
#include <stdexcept>
#include <future>
#include <chrono>
#include <optional>
#include "json.hpp"
#include "Response.h"
#include "HttpClient.h"

inline void print_ident_data(const std::string& label, const Response& data) {
    std::cout << label << " address: " << data.ip << '\n';
    if (data.hostname) {
        std::cout << "  Hostname: " << *data.hostname << '\n';
    }
    if (data.aso && data.asn) {
        std::cout << "  AS: " << *data.aso << " (" << *data.asn << ")\n";
    }
    if (data.continent) {
        std::cout << "  Continent: " << *data.continent << '\n';
    }
    if (data.country && data.cc) {
        std::cout << "  Country: " << *data.country << " (" << *data.cc << ")\n";
    }
    if (data.city && data.postal) {
        std::cout << "  City: " << *data.city << " (" << *data.postal << ")\n";
    }
    if (data.latitude && data.longitude) {
        std::cout << "  Coordinates: " << *data.latitude << ", " << *data.longitude << '\n';
    }
    if (data.tz) {
        std::cout << "  Timezone: " << *data.tz << '\n';
    }
}

Response parse_ident_json(const std::string& jsonStr) {
    try {
        nlohmann::json doc = nlohmann::json::parse(jsonStr);

        Response result;
        result.ip = doc.at("ip").get<std::string>();
        auto maybe_set_string = [&](const char* key, std::optional<std::string>& field) {
            if (doc.contains(key) && !doc[key].is_null()) {
                field = doc[key].get<std::string>();
            }
        };
        auto maybe_set_int = [&](const char* key, std::optional<int>& field) {
            if (doc.contains(key) && !doc[key].is_null()) {
                field = doc[key].get<int>();
            }
        };
        auto maybe_set_float = [&](const char* key, std::optional<float>& field) {
            if (doc.contains(key) && !doc[key].is_null()) {
                field = doc[key].get<float>();
            }
        };

        maybe_set_string("hostname",  result.hostname);
        maybe_set_string("aso",       result.aso);
        maybe_set_int("asn",          result.asn);
        maybe_set_string("continent", result.continent);
        maybe_set_string("cc",        result.cc);
        maybe_set_string("country",   result.country);
        maybe_set_string("city",      result.city);
        maybe_set_string("postal",    result.postal);
        maybe_set_float("latitude",   result.latitude);
        maybe_set_float("longitude",  result.longitude);
        maybe_set_string("tz",        result.tz);

        return result;
    } catch (const nlohmann::json::parse_error& e) {
        throw FetchError(std::string("Failed to parse JSON: ") + e.what());
    } catch (const nlohmann::json::out_of_range&) {
        // e.g. if "ip" is missing
        throw FetchError("Missing mandatory field (ip)");
    } catch (...) {
        throw FetchError("Unknown error encountered while parsing JSON");
    }
}

std::string fetchWithFallback(HttpClient& client,
                              const std::wstring& primaryHost,
                              const std::wstring& fallbackHost)
{
    try {
        return client.fetchIdent(primaryHost);
    } catch (...) {
        return client.fetchIdent(fallbackHost);
    }
}

void print_help() {
    std::cout << "Usage: ident [options]\n\n"
              << "Options:\n"
              << "  --help     Show this help message\n"
              << "  --json     Output results in JSON format\n";
    exit(0);
}

void print_json(const std::optional<Response>& v4, const std::optional<Response>& v6) {
    nlohmann::json output;

    auto to_json = [](const Response& r) {
        nlohmann::json j = {{"ip", r.ip}};
        if (r.hostname) j["hostname"] = *r.hostname;
        if (r.asn) j["asn"] = *r.asn;
        if (r.aso) j["aso"] = *r.aso;
        if (r.continent) j["continent"] = *r.continent;
        if (r.country) j["country"] = *r.country;
        if (r.cc) j["cc"] = *r.cc;
        if (r.city) j["city"] = *r.city;
        if (r.postal) j["postal"] = *r.postal;
        if (r.latitude) j["latitude"] = *r.latitude;
        if (r.longitude) j["longitude"] = *r.longitude;
        if (r.tz) j["tz"] = *r.tz;
        return j;
    };

    if (v4) output["ipv4"] = to_json(*v4);
    if (v6) output["ipv6"] = to_json(*v6);

    std::cout << output.dump(2) << '\n';
}

#ifdef WIN32
int wmain(int argc, wchar_t* argv[]) {
#else
int main(int argc, char* argv[]) {
#endif
    bool json_output = false, help = false;

    // Parse command line arguments
    for (int i = 1; i < argc; i++) {
#ifdef WIN32
        std::wstring arg(argv[i]);
        if (arg == L"--help" || arg == L"-h") help = true;
        if (arg == L"--json") json_output = true;
#else
        std::string arg(argv[i]);
        if (arg == "--help" || arg == "-h") help = true;
        if (arg == "--json") json_output = true;
#endif
    }

    if (help) {
        print_help();
        return 0;
    }

    HttpClient client;

    auto v4_future = std::async(std::launch::async, [&client] {
        return fetchWithFallback(client, L"4.ident.me", L"4.tnedi.me");
    });
    auto v6_future = std::async(std::launch::async, [&client] {
        return fetchWithFallback(client, L"6.ident.me", L"6.tnedi.me");
    });

    std::optional<Response> v4;
    std::optional<Response> v6;

    try {
        std::string v4json = v4_future.get();
        v4 = parse_ident_json(v4json);
    } catch (...) {}

    try {
        std::string v6json = v6_future.get();
        v6 = parse_ident_json(v6json);
    } catch (...) {}

    if (json_output) {
        print_json(v4, v6);
    } else {
        if (v4) { print_ident_data("IPv4", *v4); }
        else    { std::cout << "IPv4 not available.\n"; }

        if (v6) { print_ident_data("IPv6", *v6); }
        else    { std::cout << "IPv6 not available.\n"; }
    }

    return 0;
}

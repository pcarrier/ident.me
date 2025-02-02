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

        maybe_set_string("aso",       result.aso);
        maybe_set_int("asn",         result.asn);
        maybe_set_string("continent", result.continent);
        maybe_set_string("cc",       result.cc);
        maybe_set_string("country",  result.country);
        maybe_set_string("city",     result.city);
        maybe_set_string("postal",   result.postal);
        maybe_set_float("latitude",  result.latitude);
        maybe_set_float("longitude", result.longitude);
        maybe_set_string("tz",       result.tz);

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

#ifdef WIN32
int wmain() {
#else
int main() {
#endif
    const auto start = std::chrono::steady_clock::now();

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

    if (v4) { print_ident_data("IPv4", *v4); }
    else    { std::cout << "IPv4 not available.\n"; }

    if (v6) { print_ident_data("IPv6", *v6); }
    else    { std::cout << "IPv6 not available.\n"; }

    const auto end = std::chrono::steady_clock::now();
    const auto elapsed_ms =
        std::chrono::duration_cast<std::chrono::milliseconds>(end - start).count();
    std::cout << "Elapsed time: " << elapsed_ms << " ms\n";

    return 0;
}

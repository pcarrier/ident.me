#include <windows.h>
#include <winhttp.h>
#include <string>
#include <iostream>
#include <vector>
#include <stdexcept>
#include <future>
#include <chrono>
#include <optional>
#include <nlohmann/json.hpp>

#pragma comment(lib, "winhttp.lib")

// Simple RAII wrapper around a WinHTTP handle
class WinHttpHandle {
public:
    WinHttpHandle() = default;
    explicit WinHttpHandle(HINTERNET h) noexcept : handle_{h} {}
    ~WinHttpHandle() {
        if (handle_) {
            WinHttpCloseHandle(handle_);
        }
    }

    // Move-only
    WinHttpHandle(const WinHttpHandle&)            = delete;
    WinHttpHandle& operator=(const WinHttpHandle&) = delete;

    WinHttpHandle(WinHttpHandle&& other) noexcept
        : handle_{other.handle_} {
        other.handle_ = nullptr;
    }

    WinHttpHandle& operator=(WinHttpHandle&& other) noexcept {
        if (this != &other) {
            if (handle_) {
                WinHttpCloseHandle(handle_);
            }
            handle_       = other.handle_;
            other.handle_ = nullptr;
        }
        return *this;
    }

    [[nodiscard]] bool valid() const noexcept { return handle_ != nullptr; }
    [[nodiscard]] HINTERNET get() const noexcept { return handle_; }

private:
    HINTERNET handle_{nullptr};
};

// Custom exception type for network/JSON fetch errors
class FetchError : public std::runtime_error {
public:
    explicit FetchError(const std::string& msg)
        : std::runtime_error(msg) {}
};

// Response data structure
struct Response {
    std::string ip;  // Mandatory

    // Optional fields
    std::optional<std::string> aso;
    std::optional<int>         asn;
    std::optional<std::string> continent;
    std::optional<std::string> cc;
    std::optional<std::string> country;
    std::optional<std::string> city;
    std::optional<std::string> postal;
    std::optional<float>       latitude;
    std::optional<float>       longitude;
    std::optional<std::string> tz;
};

// Print only the fields that are present.
inline void print_ident_data(const std::string& label, const Response& data) {
    std::cout << label << " address: " << data.ip << '\n';

    if (data.continent) {
        std::cout << "  Continent: " << *data.continent << '\n';
    }
    if (data.aso && data.asn) {
        std::cout << "  AS: " << *data.aso << " (" << *data.asn << ")\n";
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

// Reads all response data from a WinHTTP request handle into a std::string.
std::string read_all_response_data(HINTERNET requestHandle) {
    std::string response;
    for (;;) {
        DWORD sizeAvailable = 0;
        if (!WinHttpQueryDataAvailable(requestHandle, &sizeAvailable)) {
            throw FetchError("Failed to query data availability");
        }
        if (sizeAvailable == 0) {
            break;  // No more data to read
        }
        std::vector<char> buffer(sizeAvailable);
        DWORD bytesRead = 0;
        if (!WinHttpReadData(requestHandle, buffer.data(), sizeAvailable, &bytesRead)) {
            throw FetchError("Failed to read data");
        }
        response.append(buffer.begin(), buffer.begin() + bytesRead);
    }
    return response;
}

// Parse the JSON response into a Response struct. Throws FetchError on failure.
Response parse_ident_json(const std::string& jsonStr) {
    try {
        nlohmann::json doc = nlohmann::json::parse(jsonStr);

        Response result;
        // IP is mandatory; throw if missing
        result.ip = doc.at("ip").get<std::string>();

        // Optional fields: set them only if present and not null
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

// Perform the actual WinHTTP request, read the data, parse the JSON into a Response.
Response fetch_ident_json(const std::wstring& host, const std::wstring& path = L"/json") {
    WinHttpHandle session{
        WinHttpOpen(L"identme/1.0",
                    WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                    WINHTTP_NO_PROXY_NAME,
                    WINHTTP_NO_PROXY_BYPASS,
                    0)
    };
    if (!session.valid()) {
        throw FetchError("Failed to open WinHTTP session");
    }

    WinHttpHandle connection{
        WinHttpConnect(session.get(), host.c_str(), INTERNET_DEFAULT_HTTPS_PORT, 0)
    };
    if (!connection.valid()) {
        throw FetchError("Failed to connect to host");
    }

    WinHttpHandle request{
        WinHttpOpenRequest(connection.get(),
                           L"GET",
                           path.c_str(),
                           nullptr,
                           WINHTTP_NO_REFERER,
                           WINHTTP_DEFAULT_ACCEPT_TYPES,
                           WINHTTP_FLAG_SECURE)
    };
    if (!request.valid()) {
        throw FetchError("Failed to open request");
    }

    if (!WinHttpSendRequest(request.get(),
                            WINHTTP_NO_ADDITIONAL_HEADERS, 0,
                            WINHTTP_NO_REQUEST_DATA, 0,
                            0, 0)) {
        throw FetchError("Failed to send request");
    }

    if (!WinHttpReceiveResponse(request.get(), nullptr)) {
        throw FetchError("Failed to receive response");
    }

    // Read full response data and parse it
    return parse_ident_json(read_all_response_data(request.get()));
}

// Try primaryHost first; if that fails, use fallbackHost.
Response fetch_with_fallback(const std::wstring& primaryHost,
                             const std::wstring& fallbackHost) {
    try {
        return fetch_ident_json(primaryHost);
    } catch (...) {
        return fetch_ident_json(fallbackHost);
    }
}

int wmain() {
    const auto start = std::chrono::steady_clock::now();

    // Launch two parallel tasks for IPv4 and IPv6
    auto v4_future = std::async(std::launch::async, [] {
        return fetch_with_fallback(L"4.ident.me", L"4.tnedi.me");
    });
    auto v6_future = std::async(std::launch::async, [] {
        return fetch_with_fallback(L"6.ident.me", L"6.tnedi.me");
    });

    // Optional containers for IPv4 / IPv6 responses
    std::optional<Response> v4;
    std::optional<Response> v6;

    try {
        v4 = v4_future.get();
    } catch (...) {
    }

    try {
        v6 = v6_future.get();
    } catch (...) {
    }

    if (v4) {
        print_ident_data("IPv4", *v4);
    } else {
        std::cout << "IPv4 not available.\n";
    }
    if (v6) {
        print_ident_data("IPv6", *v6);
    } else {
        std::cout << "IPv6 not available.\n";
    }

    const auto end = std::chrono::steady_clock::now();
    const auto elapsed_ms =
        std::chrono::duration_cast<std::chrono::milliseconds>(end - start).count();
    std::cout << "Elapsed time: " << elapsed_ms << " ms\n";

    return 0;
}

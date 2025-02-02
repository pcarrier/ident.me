#pragma once

#include <string>
#include <stdexcept>
#include <optional>

class FetchError : public std::runtime_error {
public:
    explicit FetchError(const std::string& msg)
        : std::runtime_error(msg) {}
};

#ifdef WIN32

#pragma comment(lib, "winhttp.lib")
#include <windows.h>
#include <winhttp.h>
#include <vector>

class WinHttpHandle {
public:
    WinHttpHandle() = default;

    explicit WinHttpHandle(HINTERNET h) noexcept
        : handle_{h} {}

    ~WinHttpHandle() {
        if (handle_) {
            WinHttpCloseHandle(handle_);
        }
    }

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

    bool valid() const noexcept {
        return handle_ != nullptr;
    }

    HINTERNET get() const noexcept {
        return handle_;
    }

private:
    HINTERNET handle_{nullptr};
};

#endif

class HttpClient {
public:
    HttpClient();
    ~HttpClient();

    std::string fetchIdent(const std::wstring& host) const;

#ifdef WIN32
private:
    WinHttpHandle session_;
#endif
};

#ifdef WIN32

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

HttpClient::HttpClient() {
    // Open a single WinHTTP session
    HINTERNET h = WinHttpOpen(L"identme/1.0",
                              WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                              WINHTTP_NO_PROXY_NAME,
                              WINHTTP_NO_PROXY_BYPASS,
                              0);
    if (!h) {
        throw FetchError("Failed to open WinHTTP session");
    }
    session_ = WinHttpHandle(h);
}

HttpClient::~HttpClient() {
}

std::string HttpClient::fetchIdent(const std::wstring& host) const {
    WinHttpHandle connection{
        WinHttpConnect(session_.get(), host.c_str(), INTERNET_DEFAULT_HTTPS_PORT, 0)
    };
    if (!connection.valid()) {
        throw FetchError("Failed to connect to host");
    }
    WinHttpHandle request{
        WinHttpOpenRequest(connection.get(),
                           L"GET",
                           L"/json",
                           nullptr,
                           WINHTTP_NO_REFERER,
                           WINHTTP_DEFAULT_ACCEPT_TYPES,
                           WINHTTP_FLAG_SECURE)
    };
    if (!request.valid()) {
        throw FetchError("Failed to open request");
    }
    if (!WinHttpSendRequest(request.get(), WINHTTP_NO_ADDITIONAL_HEADERS, 0,
                            WINHTTP_NO_REQUEST_DATA, 0, 0, 0)) {
        throw FetchError("Failed to send request");
    }
    if (!WinHttpReceiveResponse(request.get(), nullptr)) {
        throw FetchError("Failed to receive response");
    }
    return read_all_response_data(request.get());
}

#else

#include <curl/curl.h>

static size_t my_write_callback(char* ptr, size_t size, size_t nmemb, void* userdata) {
    auto* resp = reinterpret_cast<std::string*>(userdata);
    resp->append(ptr, size * nmemb);
    return size * nmemb;
}

HttpClient::HttpClient() {
    curl_global_init(CURL_GLOBAL_DEFAULT);
}

HttpClient::~HttpClient() {
    curl_global_cleanup();
}

std::string HttpClient::fetchIdent(const std::wstring& hostW) const {
    std::string host(hostW.begin(), hostW.end());
    std::string url = "https://" + host + "/json";
    std::string responseData;

    CURL* curl = curl_easy_init();
    if (!curl) {
        throw FetchError("Failed to initialize cURL");
    }
    curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
    curl_easy_setopt(curl, CURLOPT_USERAGENT, "identme/1.0");
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, my_write_callback);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, &responseData);

    CURLcode res = curl_easy_perform(curl);
    if (res != CURLE_OK) {
        curl_easy_cleanup(curl);
        throw FetchError(std::string("cURL request failed: ") + curl_easy_strerror(res));
    }
    long httpStatus = 0;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &httpStatus);
    curl_easy_cleanup(curl);

    if (httpStatus < 200 || httpStatus >= 300) {
        throw FetchError("HTTP status code " + std::to_string(httpStatus));
    }

    return responseData;
}

#endif

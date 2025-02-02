#pragma once

#include <string>
#include <optional>

struct Response {
    std::string ip;
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

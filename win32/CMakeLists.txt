cmake_minimum_required(VERSION 3.10)
project(identme)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_BUILD_TYPE "MinSizeRel" CACHE STRING "Build type" FORCE)
add_executable(identme main.cpp)
target_link_libraries(identme PRIVATE Winhttp)
if(MSVC)
    target_compile_options(identme PRIVATE /EHsc)
    target_compile_options(identme PRIVATE /O1 /GL)
    set_property(TARGET identme PROPERTY LINK_FLAGS "/LTCG /OPT:REF /OPT:ICF")
else()
    target_compile_options(identme PRIVATE -Os -flto -s)
    set(CMAKE_INTERPROCEDURAL_OPTIMIZATION TRUE)
endif()
include(FetchContent)
FetchContent_Declare(
    nlohmann_json
    GIT_REPOSITORY https://github.com/nlohmann/json.git
    GIT_TAG        v3.11.3
)
FetchContent_MakeAvailable(nlohmann_json)
target_link_libraries(identme PRIVATE nlohmann_json::nlohmann_json)
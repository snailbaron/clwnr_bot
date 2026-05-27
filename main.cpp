#include <tgbot/tgbot.h>

#include <chrono>
#include <cstdlib>
#include <exception>
#include <iostream>
#include <random>

class ClownToss {
public:
    ClownToss()
        : _engine(std::mt19937{_rd()})
    { }

    void setClownProbability(float p)
    {
        _clownProbability = p;
    }

    bool operator()()
    {
        auto timestamp = std::chrono::system_clock::now();
        auto toss = _dist(_engine);

        std::cout << "[" << timestamp << "] ";
        if (toss <= _clownProbability) {
            std::cout << "toss pass: " << toss << "\n";
            return true;
        }
        std::cout << "toss fail: " << toss << "\n";
        return false;
    }

private:
    std::random_device _rd;
    std::mt19937 _engine;
    std::uniform_real_distribution<float> _dist;
    float _clownProbability = 1.f / 100;
};

int main() {
    std::cout.setf(std::ios::unitbuf);

    const char* token = std::getenv("TOKEN");
    if (!token) {
        std::cerr << "Fill TOKEN environment variable\n";
        return EXIT_FAILURE;
    }

    auto toss = ClownToss{};
    auto bot = TgBot::Bot{token};

    auto commands = std::vector<TgBot::BotCommand::Ptr>{};
    {
        auto stopCommand = TgBot::BotCommand::Ptr{new TgBot::BotCommand};
        stopCommand->command = "stop";
        stopCommand->description = "Попроси клоунщика перестать";

        auto pCommand = TgBot::BotCommand::Ptr{new TgBot::BotCommand};
        pCommand->command = "p";
        pCommand->description =
            "Установи вероятность клоунения сообщений, в процентах";
    }
    bot.getApi().setMyCommands(commands);

    bot.getEvents().onCommand(
        "stop",
        [&bot] (const TgBot::Message::Ptr& message) {
            bot.getApi().sendMessage(message->chat->id, "Нет!");
        });
    bot.getEvents().onCommand(
        "p",
        [&bot, &toss] (const TgBot::Message::Ptr& message) {
            try {
                auto stream = std::istringstream{message->text};
                stream.exceptions(std::ios::badbit | std::ios::failbit);
                std::string s;

                stream >> s;
                int percent = 0;
                stream >> percent;

                if (percent < 1) {
                    bot.getApi().sendMessage(
                        message->chat->id,
                        "Это слишком мало! Я поставлю 1%.",
                        false,
                        message->messageId);
                    percent = 1;
                } else if (percent > 100) {
                    bot.getApi().sendMessage(
                        message->chat->id,
                        "Столько не бывает! Я поставлю 100%.",
                        false,
                        message->messageId);
                    percent = 100;
                } else {
                    bot.getApi().sendMessage(
                        message->chat->id,
                        "Ставлю клоунение в " + std::to_string(percent) + "%.",
                        false,
                        message->messageId);
                }

                toss.setClownProbability(percent / 100.f);
            } catch (const std::exception&) {
                bot.getApi().sendMessage(
                    message->chat->id,
                    "Что-то здесь не так: " + message->text,
                    false,
                    message->messageId);
            }
        });
    bot.getEvents().onAnyMessage(
        [&bot, &toss] (const TgBot::Message::Ptr& message) {
            if (toss()) {
                bot.getApi().sendMessage(message->chat->id, "\xF0\x9F\xA4\xA1", false, message->messageId);
            }
        });

    try {
        auto longPoll = TgBot::TgLongPoll{bot};
        for (;;) {
            std::cout << "polling\n";
            longPoll.start();
        }
    } catch (const std::exception& e) {
        std::cerr << e.what();
    }
}

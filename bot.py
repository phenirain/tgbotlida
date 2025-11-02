import os
import re
import sqlite3
import logging
from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import (
    Application,
    CommandHandler,
    CallbackQueryHandler,
    MessageHandler,
    filters,
    ContextTypes,
    ConversationHandler,
)

# Configure logging
logging.basicConfig(
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    level=logging.INFO
)
logger = logging.getLogger(__name__)

# Conversation states
AWAITING_EMAIL = 1

# Environment variables
BOT_TOKEN = os.getenv('BOT_TOKEN')
WELCOME_MESSAGE = os.getenv('WELCOME_MESSAGE', 'Welcome to our bot!')
POLICY_TEXT = os.getenv('POLICY_TEXT', 'Please accept our privacy policy to continue.')
FINAL_MESSAGE = os.getenv('FINAL_MESSAGE', 'Thank you! Your email has been saved.')
DB_PATH = os.getenv('DB_PATH', 'users.db')


def init_db():
    """Initialize SQLite database"""
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute('''
        CREATE TABLE IF NOT EXISTS users (
            user_id INTEGER PRIMARY KEY,
            email TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    ''')
    conn.commit()
    conn.close()


def is_valid_email(email: str) -> bool:
    """Validate email format"""
    pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
    return re.match(pattern, email) is not None


def user_exists(user_id: int) -> bool:
    """Check if user already submitted email"""
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute('SELECT 1 FROM users WHERE user_id = ?', (user_id,))
    exists = cursor.fetchone() is not None
    conn.close()
    return exists


def save_user_email(user_id: int, email: str):
    """Save user email to database"""
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute('INSERT INTO users (user_id, email) VALUES (?, ?)', (user_id, email))
    conn.commit()
    conn.close()


async def start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Handle /start command"""
    user_id = update.effective_user.id

    # Check if user already submitted email
    if user_exists(user_id):
        await update.message.reply_text(
            "You have already submitted your email. Thank you!"
        )
        return ConversationHandler.END

    # Send welcome message
    await update.message.reply_text(WELCOME_MESSAGE)

    # Show policy acceptance button
    keyboard = [[InlineKeyboardButton("Accept", callback_data='accept_policy')]]
    reply_markup = InlineKeyboardMarkup(keyboard)

    await update.message.reply_text(
        POLICY_TEXT,
        reply_markup=reply_markup
    )

    return AWAITING_EMAIL


async def policy_accepted(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Handle policy acceptance"""
    query = update.callback_query
    await query.answer()

    user_id = update.effective_user.id

    # Check again if user already submitted
    if user_exists(user_id):
        await query.edit_message_text(
            "You have already submitted your email. Thank you!"
        )
        return ConversationHandler.END

    await query.edit_message_text("Policy accepted!")
    await query.message.reply_text("Please provide your email address:")

    return AWAITING_EMAIL


async def receive_email(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Handle email submission"""
    user_id = update.effective_user.id
    email = update.message.text.strip()

    # Check if user already submitted
    if user_exists(user_id):
        await update.message.reply_text(
            "You have already submitted your email. Thank you!"
        )
        return ConversationHandler.END

    # Validate email
    if not is_valid_email(email):
        await update.message.reply_text(
            "Invalid email format. Please provide a valid email address:"
        )
        return AWAITING_EMAIL

    # Save email
    try:
        save_user_email(user_id, email)
        await update.message.reply_text(FINAL_MESSAGE)
        return ConversationHandler.END
    except Exception as e:
        logger.error(f"Error saving email: {e}")
        await update.message.reply_text(
            "An error occurred. Please try again later."
        )
        return ConversationHandler.END


async def cancel(update: Update, context: ContextTypes.DEFAULT_TYPE):
    """Handle conversation cancellation"""
    await update.message.reply_text("Operation cancelled.")
    return ConversationHandler.END


def main():
    """Start the bot"""
    if not BOT_TOKEN:
        raise ValueError("BOT_TOKEN environment variable is required")

    # Initialize database
    init_db()

    # Create application
    application = Application.builder().token(BOT_TOKEN).build()

    # Setup conversation handler
    conv_handler = ConversationHandler(
        entry_points=[CommandHandler('start', start)],
        states={
            AWAITING_EMAIL: [
                CallbackQueryHandler(policy_accepted, pattern='^accept_policy$'),
                MessageHandler(filters.TEXT & ~filters.COMMAND, receive_email),
            ],
        },
        fallbacks=[CommandHandler('cancel', cancel)],
    )

    application.add_handler(conv_handler)

    # Start bot
    logger.info("Bot started")
    application.run_polling(allowed_updates=Update.ALL_TYPES)


if __name__ == '__main__':
    main()

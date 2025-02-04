console.log("JavaScript loaded"); // Проверка загрузки скрипта
console.log("✅ script.js загружен!");

document.addEventListener("DOMContentLoaded", function () {
    console.log("✅ script.js загружен и выполняется");
    const registerForm = document.getElementById("registerForm");
    console.log("🔍 Найдена ли форма:", registerForm);


    const name = document.getElementById("name").value;
    const email = document.getElementById("email").value;
    const password = document.getElementById("password").value;
    const role = document.getElementById("role").value; // Получаем роль из формы
    console.log("📤 Отправляем данные на сервер:", { name, email, password, role });

    if (registerForm) {
        console.log("✅ Форма регистрации найдена!");

        registerForm.addEventListener("submit", async function (event) {
            
            console.log("✅ Обработчик submit сработал!");
            const formData = new FormData(registerForm);
            const data = Object.fromEntries(formData.entries());

            if (data.password !== data.confirmPassword) {
                alert("Passwords do not match");
                return;
            }

            console.log("Отправка данных на сервер:", data);

            try {
                const response = await fetch("/register", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        name: data.name,
                        email: data.email,
                        password: data.password,
                        role: data.role,
                    }),
                });

                const result = await response.json();
                console.log("Ответ сервера:", result);

                document.getElementById("statusMessage").innerText = result.message;
            } catch (error) {
                console.error("Ошибка регистрации:", error);
            }
        });
    }
});
document.addEventListener("DOMContentLoaded", function () {
    const loginForm = document.getElementById("loginForm");

    if (loginForm) {
        loginForm.addEventListener("submit", async function (event) {
            event.preventDefault();

            const email = document.getElementById("email").value;
            const password = document.getElementById("password").value;

            console.log("📤 Отправка запроса на сервер:", {
                method: "POST",
                url: "http://localhost:8080/login",
                body: { email, password }
            });

            try {
                const response = await fetch("http://localhost:8080/login", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ email, password }),
                });

                console.log("📥 Ответ сервера:", response);

                const result = await response.json();
                console.log("📩 JSON-ответ сервера:", result);

                if (response.ok) {
                    localStorage.setItem("token", result.token);
                    document.getElementById("statusMessage").innerText = "✅ Login successful! Redirecting...";
                    setTimeout(() => {
                        window.location.href = "me.html";
                    }, 1000);
                } else {
                    document.getElementById("statusMessage").innerText = "❌ " + (result.error || "Login failed");
                }
            } catch (error) {
                console.error("❌ Ошибка входа:", error);
                document.getElementById("statusMessage").innerText = "❌ Ошибка соединения с сервером";
            }
        });
    }

    // Проверка токена и редирект
    if (window.location.pathname.endsWith("me.html")) {
        fetchProfile();
    }

    // Обработчик выхода (Logout)
    const logoutButton = document.getElementById("logoutButton");
    if (logoutButton) {
        logoutButton.addEventListener("click", function () {
            localStorage.removeItem("token");
            window.location.href = "signin.html"; // Перенаправление на страницу входа
        });
    }
});

// Функция загрузки профиля
// async function fetchProfile() {
//     const token = localStorage.getItem("token");

//     if (!token) {
//         console.log("❌ Пользователь не авторизован. Перенаправляем на страницу входа.");
//         window.location.href = "account.html";
//         return;
//     }

//     try {
//         const response = await fetch("http://localhost:8080/api/profile", {
//             method: "GET",
//             headers: { Authorization: `Bearer ${token}` },
//         });

//         if (!response.ok) {
//             console.log("❌ Ошибка загрузки профиля. Перенаправляем на вход...");
//             localStorage.removeItem("token");
//             window.location.href = "account.html";
//             return;
//         }

//         const data = await response.json();
//         console.log("✅ Данные профиля:", data);

//         document.getElementById("profileInfo").innerText = `Добро пожаловать, ${data.message}`;

//         // Проверяем роль пользователя
//         if (data.role === "admin") {
//             document.getElementById("adminSection").style.display = "block";
//         } else {
//             document.getElementById("adminSection").style.display = "none";
//         }
//     } catch (error) {
//         console.error("❌ Ошибка загрузки профиля:", error);
//         document.getElementById("profileInfo").innerText = "Ошибка загрузки профиля!";
//     }
//     document.addEventListener("DOMContentLoaded", fetchProfile);
// }
async function fetchProfile() {
    const token = localStorage.getItem("token");

    if (!token) {
        console.log("❌ Пользователь не авторизован. Перенаправляем на страницу входа.");
        window.location.href = "account.html";
        return;
    }

    try {
        const response = await fetch("http://localhost:8080/api/profile", {
            method: "GET",
            headers: { Authorization: `Bearer ${token}` },
        });

        console.log("📡 Ответ сервера:", response.status); // Логируем статус ответа

        if (!response.ok) {
            console.log("❌ Ошибка загрузки профиля. Перенаправляем на вход...");
            localStorage.removeItem("token");
            window.location.href = "account.html";
            return;
        }

        let data;
        try {
            data = await response.json();
        } catch (jsonError) {
            console.error("❌ Ошибка обработки JSON:", jsonError);
            document.getElementById("profileInfo").innerText = "Ошибка загрузки профиля!";
            return;
        }

        console.log("✅ Данные профиля:", data);

        document.getElementById("profileInfo").innerText = `Добро пожаловать, ${data.message}`;

        // Проверяем роль пользователя
        const adminSection = document.getElementById("adminSection");
        if (adminSection) {
            adminSection.style.display = data.role === "admin" ? "block" : "none";
        }
    } catch (error) {
        console.error("❌ Ошибка сети при загрузке профиля:", error);
        document.getElementById("profileInfo").innerText = "Ошибка сети!";
    }
}

// Вызываем `fetchProfile()` при загрузке страницы
document.addEventListener("DOMContentLoaded", fetchProfile);




// Находим кнопку и меню
const accountToggle = document.querySelector('.account-toggle');
const accountMenu = document.querySelector('.account-menu');

// Добавляем обработчик клика
accountToggle.addEventListener('click', (e) => {
  e.preventDefault(); // Предотвращаем переход по ссылке
  
  // Получаем размеры и положение кнопки
  const rect = accountToggle.getBoundingClientRect();
  
  // Устанавливаем позицию меню относительно кнопки
  accountMenu.style.left = `${rect.left}px`;  // Выравнивание по левому краю кнопки
  accountMenu.style.top = `${rect.bottom + window.scrollY}px`;  // Вычисление нижнего отступа
  
  // Переключаем видимость меню
  accountMenu.style.display = accountMenu.style.display === 'block' ? 'none' : 'block';
});

// Закрытие меню при клике вне его области
document.addEventListener('click', (e) => {
  if (!accountToggle.contains(e.target) && !accountMenu.contains(e.target)) {
    accountMenu.style.display = 'none';
  }
});


////
// Получение элементов
const createBookBtn = document.getElementById('create-book-btn');
const readBooksBtn = document.getElementById('read-books-btn');
const updateBookBtn = document.getElementById('update-book-btn');
const deleteBookBtn = document.getElementById('delete-book-btn');
const booksTable = document.getElementById('books-table').getElementsByTagName('tbody')[0];

// Функция для обновления таблицы
function updateBooksTable(books) {
    booksTable.innerHTML = ''; // Очистить таблицу
    books.forEach(book => {
        const row = booksTable.insertRow();
        row.innerHTML = `
            <td>${book.id}</td>
            <td>${book.title}</td>
            <td>${book.author}</td>
            <td>${book.published}</td>
        `;
    });
}

// Функция для получения всех книг
async function fetchBooks() {
    try {
        const response = await fetch('http://localhost:8080/books'); // Адрес вашего сервера
        const books = await response.json();
        updateBooksTable(books);
    } catch (error) {
        console.error('Error fetching books:', error);
    }
}

// Обработчик для кнопки "View All Books"
readBooksBtn.addEventListener('click', fetchBooks);

// Обработчик для кнопки "Add a New Book"
createBookBtn.addEventListener('click', async () => {
    const title = prompt("Enter book title:");
    const author = prompt("Enter book author:");
    const published = prompt("Enter book published date (YYYY-MM-DD):");

    const errorContainer = document.getElementById('error-container'); // Контейнер для ошибок

    // Очищаем ошибки перед новой проверкой
    errorContainer.textContent = '';
    errorContainer.style.display = 'none';

    // Проверка, заполнены ли все поля
    if (!title || !author || !published) {
        showError("All fields are required!");
        return;
    }

    const newBook = { title, author, published };

    try {
        const response = await fetch('http://localhost:8080/books/add', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(newBook),
        });

        if (response.ok) {
            alert('Book added successfully!');
            fetchBooks(); // Обновить список книг
        } else {
            // Если сервер вернул ошибку, извлекаем её из JSON-ответа
            const errorData = await response.json();
            showError(`Failed to add the book: ${errorData.error}`);
        }
    } catch (error) {
        console.error('Error adding book:', error);
        showError('An unexpected error occurred while adding the book.');
    }
});

// Функция для отображения ошибок
function showError(message) {
    const errorContainer = document.getElementById('error-container');
    errorContainer.textContent = message;
    errorContainer.style.display = 'block';
}

// Обработчик для кнопки "Update Book"
updateBookBtn.addEventListener('click', async () => {
    const id = prompt("Enter book ID to update:");
    const title = prompt("Enter new book title:");
    const author = prompt("Enter new book author:");
    const published = prompt("Enter new published date (YYYY-MM-DD):");

    if (id && title && author && published) {
        // Добавляем id в объект
        const updatedBook = { id: parseInt(id), title, author, published };

        try {
            const response = await fetch("http://localhost:8080/books/update", {
                method: 'PUT', // Метод PUT для обновления
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(updatedBook), // Тело запроса
            });

            if (response.ok) {
                alert('Book updated successfully!');
                fetchBooks(); // Обновить список книг
            } else {
                const errorText = await response.text();
                alert("Failed to update the book: ${errorText}");
            }
        } catch (error) {
            console.error('Error updating book:', error);
        }
    } else {
        alert('All fields are required!');
    }
});

// Обработчик для кнопки "Delete Book"
deleteBookBtn.addEventListener('click', async () => {
    const id = prompt("Enter book ID to delete:");

    if (id) {
        try {
            const response = await fetch(`http://localhost:8080/books/delete?id=${id}`, {
                method: 'DELETE',
            });

            if (response.ok) {
                alert('Book deleted successfully!');
                fetchBooks(); // Обновить список книг
            } else {
                alert('Failed to delete the book.');
            }
        } catch (error) {
            console.error('Error deleting book:', error);
        }
    } else {
        alert('Book ID is required!');
    }
});
searchBookBtn.addEventListener('click', async () => {
    const id = prompt("Enter book ID to search:");

    if (id) {
        try {
            const response = await fetch(`http://localhost:8080/books/search?id=${id}`);

            if (response.ok) {
                const book = await response.json();
                alert(`Book found: \nTitle: ${book.title}\nAuthor: ${book.author}\nPublished: ${book.published}`);
            } else {
                alert('Book not found.');
            }
        } catch (error) {
            console.error('Error searching for book:', error);
        }
    } else {
        alert('ID is required!');
    }
});

document.getElementById('apply-filters').addEventListener('click', async () => {
    const title = document.getElementById('filter-title').value;
    const author = document.getElementById('filter-author').value;
    const published = document.getElementById('filter-published').value;
    const sortBy = document.getElementById('sort-by').value;
    const sortOrder = document.getElementById('sort-order').value;

    const params = new URLSearchParams();
    if (title) params.append('title', title);
    if (author) params.append('author', author);
    if (published) params.append('published', published);
    if (sortBy) params.append('sortBy', sortBy);
    if (sortOrder) params.append('sortOrder', sortOrder);

    try {
        const response = await fetch(`http://localhost:8080/books?${params.toString()}`);
        const books = await response.json();
        updateBooksTable(books);  // Функция для обновления таблицы
    } catch (error) {
        console.error('Error fetching books with filters:', error);
    }
});


// Получение элементов


// Получение данных из API и отображение карточек на странице
document.addEventListener('DOMContentLoaded', () => {
    const container = document.getElementById('fantasy-books-container');

    // Функция для создания карточки книги
    const createBookCard = (book) => {
        const card = document.createElement('div');
        card.className = 'book-card';

        card.innerHTML = `
            <img src="${book.image_url}" alt="${book.title}" class="book-image">
            <h3 class="book-title">${book.title}</h3>
            <p class="book-description">${book.description}</p>
            <p class="book-price">Price: $${book.price.toFixed(2)}</p>
        `;

        return card;
    };

    // Функция для загрузки данных из API
    const loadFantasyBooks = async () => {
        try {
            const response = await fetch('/fantasy/books');
            if (!response.ok) {
                throw new Error('Failed to fetch fantasy books');
            }

            const books = await response.json();
            books.forEach((book) => {
                const bookCard = createBookCard(book);
                container.appendChild(bookCard);
            });
        } catch (error) {
            console.error('Error loading fantasy books:', error);
            container.innerHTML = '<p>Failed to load fantasy books. Please try again later.</p>';
        }
    };

    loadFantasyBooks();
});

// Функция для отправки сообщения
async function sendMessage(event) {
    console.log("SendMessage function triggered"); 
    event.preventDefault(); 

    const form = event.target; 
    const formData = new FormData(form); 
    const notification = document.getElementById('notification'); 

    try {
        // Отправляем запрос на сервер
        const response = await fetch(form.action, { 
            method: form.method, 
            body: formData,
        });

        if (response.ok) {
            const data = await response.json(); 
            notification.textContent = data.status; 
            notification.style.color = 'green'; 
            form.reset(); 
        } else {
            const errorData = await response.text(); 
            notification.textContent = `Failed to send message: ${errorData}`; 
            notification.style.color = 'red'; 
        }
    } catch (error) {
        console.error('Error sending message:', error); 
        notification.textContent = 'An unexpected error occurred while sending the message.'; 
        notification.style.color = 'red'; 

    // Показываем уведомление
    notification.style.display = 'block';

    // Скрываем уведомление через 5 секунд (по желанию)
    setTimeout(() => {
        notification.style.display = 'none';
    }, 5000);

    // Привязываем обработчик события submit
document.addEventListener('DOMContentLoaded', () => {
    console.log("DOM fully loaded and parsed"); // Для отладки
    const sendMessageForm = document.getElementById('sendMessageForm');
    if (sendMessageForm) {
        console.log("Form found and event listener added"); 
        sendMessageForm.addEventListener('submit', sendMessage);
    } else {
        console.error("Form not found!"); 
    }
});
}
}

document.addEventListener("DOMContentLoaded", function () {
    // Получение списка пользователей
    fetchUsers();

    // Функция для получения списка пользователей
    function fetchUsers() {
        fetch("/admin/users", {
            method: "GET",
            headers: {
                "Role": "admin" // Укажите здесь способ передачи роли
            }
        })
            .then(response => {
                if (!response.ok) {
                    throw new Error("Failed to fetch users");
                }
                return response.json();
            })
            .then(users => {
                const tbody = document.querySelector("#users-table tbody");
                tbody.innerHTML = ""; // Очистка таблицы
                users.forEach(user => {
                    const row = document.createElement("tr");
                    row.innerHTML = `
                        <td>${user.id}</td>
                        <td>${user.email}</td>
                        <td>${user.role}</td>
                        <td>
                            <button onclick="changeRole(${user.id}, 'admin')">Make Admin</button>
                            <button onclick="changeRole(${user.id}, 'user')">Make User</button>
                            <button onclick="deleteUser(${user.id})">Delete</button>
                        </td>
                    `;
                    tbody.appendChild(row);
                });
            })
            .catch(error => console.error(error));
    }

    // Функция для изменения роли пользователя
    window.changeRole = function (userId, newRole) {
        fetch("/admin/users/update-role", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Role": "admin"
            },
            body: JSON.stringify({ user_id: userId, role: newRole })
        })
            .then(response => {
                if (!response.ok) {
                    throw new Error("Failed to update user role");
                }
                return response.json();
            })
            .then(data => {
                alert(data.message);
                fetchUsers();
            })
            .catch(error => console.error(error));
    };

    // Функция для удаления пользователя
    window.deleteUser = function (userId) {
        fetch(`/admin/users/delete?id=${userId}`, {
            method: "DELETE",
            headers: {
                "Role": "admin"
            }
        })
            .then(response => {
                if (!response.ok) {
                    throw new Error("Failed to delete user");
                }
                return response.json();
            })
            .then(data => {
                alert(data.message);
                fetchUsers();
            })
            .catch(error => console.error(error));
    };
});
////FOR ACCOUNT.HTML
//////FOR SIGNIN
// script.js
